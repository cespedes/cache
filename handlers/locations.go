package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cespedes/cache/db"
	"github.com/cespedes/cache/models"
	"github.com/jackc/pgx/v5"
)

func getLocationsNewPosition(before, after *int) (float64, error) {
	var newPosition float64
	var posBefore, posAfter *float64
	switch {
	case before != nil && after != nil:
		return 0, fmt.Errorf("cannot specify both `before` and `after` in location")
	case before != nil:
		query := `
			WITH foo AS (
				SELECT position
				FROM locations
				WHERE id=$1
			) SELECT
				(SELECT position AS after FROM foo),
				(SELECT COALESCE(max(position),0) AS before
					FROM locations
					WHERE position < (SELECT position FROM foo))
		`
		err := db.Pool.QueryRow(context.Background(), query, *before).Scan(&posBefore, &posAfter)
		if err != nil {
			return 0, fmt.Errorf("getting position before id %d: %w", *before, err)
		}
		if posAfter == nil || posBefore == nil {
			return 0, fmt.Errorf("location %d not found", *before)
		}
		newPosition = (*posBefore + *posAfter) / 2
	case after != nil:
		query := `
			WITH foo AS (
				SELECT position
				FROM locations
				WHERE id=$1
			) SELECT
				(SELECT position AS before FROM foo),
				(SELECT COALESCE(min(position),(SELECT(max(position)+2000) FROM locations)) AS before
					FROM locations
					WHERE position > (SELECT position FROM foo))
		`
		err := db.Pool.QueryRow(context.Background(), query, *after).Scan(&posBefore, &posAfter)
		if err != nil {
			return 0, fmt.Errorf("getting position after id %d: %w", *after, err)
		}
		if posAfter == nil || posBefore == nil {
			return 0, fmt.Errorf("location %d not found", *after)
		}
		newPosition = (*posBefore + *posAfter) / 2
	default:
		// both before and after are nil: return a position at the end
		query := `SELECT COALESCE(MAX(position),0) + 1000 FROM locations`
		err := db.Pool.QueryRow(context.Background(), query).Scan(&newPosition)
		if err != nil {
			return 0, fmt.Errorf("getting last position of attributes: %w", err)
		}
	}
	return newPosition, nil
}

func listLocationsFromRows(w http.ResponseWriter, rows pgx.Rows) ([]models.Location, error) {
	locations := []models.Location{}
	for rows.Next() {
		var loc models.Location
		if err := rows.Scan(&loc.ID, &loc.Name, &loc.ParentID, &loc.CreatedAt, &loc.UpdatedAt); err != nil {
			return nil, err
		}
		locations = append(locations, loc)
	}
	return locations, rows.Err()
}

func ListLocations(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")

	var (
		rows pgx.Rows
		err  error
	)

	if search != "" {
		rows, err = db.Pool.Query(context.Background(), `
			SELECT id, name, parent_id, created_at, updated_at
			FROM locations
			WHERE name ILIKE '%' || $1 || '%'
			ORDER BY position
		`, search)
	} else {
		rows, err = db.Pool.Query(context.Background(), `
			SELECT id, name, parent_id, created_at, updated_at
			FROM locations
			ORDER BY position
		`)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	locations, err := listLocationsFromRows(w, rows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(locations)
}

func GetLocation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var loc models.Location
	err = db.Pool.QueryRow(context.Background(),
		`SELECT id, name, parent_id, created_at, updated_at FROM locations WHERE id = $1`, id).
		Scan(&loc.ID, &loc.Name, &loc.ParentID, &loc.CreatedAt, &loc.UpdatedAt)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loc)
}

func CreateLocation(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		ParentID *int   `json:"parent_id"`
		Before   *int   `json:"before,omitempty"` // for ordering purposes
		After    *int   `json:"after,omitempty"`  // for ordering purposes
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	position, err := getLocationsNewPosition(input.Before, input.After)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var loc models.Location
	err = db.Pool.QueryRow(context.Background(), `
		INSERT INTO locations (name, parent_id, position) VALUES ($1, $2, $3)
		RETURNING id, name, parent_id, created_at, updated_at
	`, input.Name, input.ParentID, position).
		Scan(&loc.ID, &loc.Name, &loc.ParentID, &loc.CreatedAt, &loc.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(loc)
}

func UpdateLocation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var input struct {
		Name     string `json:"name"`
		ParentID *int   `json:"parent_id"`
		Before   *int   `json:"before,omitempty"` // for ordering purposes
		After    *int   `json:"after,omitempty"`  // for ordering purposes
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var position float64
	if input.Before != nil || input.After != nil {
		position, err = getLocationsNewPosition(input.Before, input.After)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		err = db.Pool.QueryRow(context.Background(), `
			SELECT position
			FROM locations
			WHERE id=$1
		`, id).Scan(&position)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var loc models.Location
	err = db.Pool.QueryRow(context.Background(), `
		UPDATE locations SET name=$1, parent_id=$2, position=$3
		WHERE id=$4
		RETURNING id, name, parent_id, created_at, updated_at
	`, input.Name, input.ParentID, position, id).
		Scan(&loc.ID, &loc.Name, &loc.ParentID, &loc.CreatedAt, &loc.UpdatedAt)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loc)
}

func DeleteLocation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	result, err := db.Pool.Exec(context.Background(), `
		DELETE FROM locations WHERE id = $1
	`, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if result.RowsAffected() == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
