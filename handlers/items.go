package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cespedes/cache/db"
	"github.com/cespedes/cache/models"
)

func getItemsNewPosition(before, after *int) (float64, error) {
	var newPosition float64
	var posBefore, posAfter *float64
	switch {
	case before != nil && after != nil:
		return 0, fmt.Errorf("cannot specify both `before` and `after` in item")
	case before != nil:
		query := `
			WITH foo AS (
				SELECT position
				FROM items
				WHERE id=$1
			) SELECT
				(SELECT position AS after FROM foo),
				(SELECT COALESCE(max(position),0) AS before
					FROM items
					WHERE position < (SELECT position FROM foo))
		`
		err := db.Pool.QueryRow(context.Background(), query, *before).Scan(&posBefore, &posAfter)
		if err != nil {
			return 0, fmt.Errorf("getting position before id %d: %w", *before, err)
		}
		if posAfter == nil || posBefore == nil {
			return 0, fmt.Errorf("item %d not found", *before)
		}
		newPosition = (*posBefore + *posAfter) / 2
	case after != nil:
		query := `
			WITH foo AS (
				SELECT position
				FROM items
				WHERE id=$1
			) SELECT
				(SELECT position AS before FROM foo),
				(SELECT COALESCE(min(position),(SELECT(max(position)+2000) FROM items)) AS before
					FROM items
					WHERE position > (SELECT position FROM foo))
		`
		err := db.Pool.QueryRow(context.Background(), query, *after).Scan(&posBefore, &posAfter)
		if err != nil {
			return 0, fmt.Errorf("getting position after id %d: %w", *after, err)
		}
		if posAfter == nil || posBefore == nil {
			return 0, fmt.Errorf("item %d not found", *after)
		}
		newPosition = (*posBefore + *posAfter) / 2
	default:
		// both before and after are nil: return a position at the end
		query := `SELECT COALESCE(MAX(position),0) + 1000 FROM items`
		err := db.Pool.QueryRow(context.Background(), query).Scan(&newPosition)
		if err != nil {
			return 0, fmt.Errorf("getting last position of attributes: %w", err)
		}
	}
	return newPosition, nil
}

func ListItems(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("q")
	locationStr := r.URL.Query().Get("location_id")

	query := `SELECT id, name, location_id, created_at, updated_at FROM items WHERE 1=1`
	args := []any{}
	argIdx := 1

	if search != "" {
		query += ` AND name ILIKE '%' || $` + strconv.Itoa(argIdx) + ` || '%'`
		args = append(args, search)
		argIdx++
	}
	if locationStr != "" {
		locationID, err := strconv.Atoi(locationStr)
		if err != nil {
			http.Error(w, "invalid location_id", http.StatusBadRequest)
			return
		}
		query += ` AND location_id = $` + strconv.Itoa(argIdx)
		args = append(args, locationID)
	}
	query += ` ORDER BY position`

	rows, err := db.Pool.Query(context.Background(), query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []models.Item{}
	for rows.Next() {
		var item models.Item
		if err := rows.Scan(&item.ID, &item.Name, &item.LocationID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		http.Error(w, rows.Err().Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func GetItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var item models.Item
	err = db.Pool.QueryRow(context.Background(),
		`SELECT id, name, location_id, created_at, updated_at FROM items WHERE id = $1`, id).
		Scan(&item.ID, &item.Name, &item.LocationID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func CreateItem(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name       string `json:"name"`
		LocationID int    `json:"location_id"`
		Before     *int   `json:"before,omitempty"` // for ordering purposes
		After      *int   `json:"after,omitempty"`  // for ordering purposes
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name == "" || input.LocationID == 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	position, err := getItemsNewPosition(input.Before, input.After)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var item models.Item
	err = db.Pool.QueryRow(context.Background(), `
		INSERT INTO items (name, location_id, position) VALUES ($1, $2, $3)
		RETURNING id, name, location_id, created_at, updated_at
	`, input.Name, input.LocationID, position).
		Scan(&item.ID, &item.Name, &item.LocationID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func UpdateItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var input struct {
		Name       string `json:"name"`
		LocationID int    `json:"location_id"`
		Before     *int   `json:"before,omitempty"` // for ordering purposes
		After      *int   `json:"after,omitempty"`  // for ordering purposes
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name == "" || input.LocationID == 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var position float64
	if input.Before != nil || input.After != nil {
		position, err = getItemsNewPosition(input.Before, input.After)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		err = db.Pool.QueryRow(context.Background(), `
			SELECT position
			FROM items
			WHERE id=$1
		`, id).Scan(&position)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	var item models.Item
	err = db.Pool.QueryRow(context.Background(), `
		UPDATE items SET name=$1, location_id=$2, position=$3
		WHERE id=$4
		RETURNING id, name, location_id, created_at, updated_at
	`, input.Name, input.LocationID, position, id).
		Scan(&item.ID, &item.Name, &item.LocationID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func DeleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	result, err := db.Pool.Exec(context.Background(),
		`DELETE FROM items WHERE id = $1`, id)
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
