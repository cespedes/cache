package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cespedes/cache/db"
	"github.com/cespedes/cache/models"
	"github.com/jackc/pgx/v5"
)

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
		rows, err = db.Pool.Query(context.Background(),
			`SELECT id, name, parent_id, created_at, updated_at
																																											 FROM locations WHERE name ILIKE '%' || $1 || '%' ORDER BY name`,
			search)
	} else {
		rows, err = db.Pool.Query(context.Background(),
			`SELECT id, name, parent_id, created_at, updated_at FROM locations ORDER BY name`)
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
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var loc models.Location
	err := db.Pool.QueryRow(context.Background(),
		`INSERT INTO locations (name, parent_id) VALUES ($1, $2)
																																																																																																																				 RETURNING id, name, parent_id, created_at, updated_at`,
		input.Name, input.ParentID).
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
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var loc models.Location
	err = db.Pool.QueryRow(context.Background(),
		`UPDATE locations SET name = $1, parent_id = $2
																																																																																																																																																														 WHERE id = $3
																																																																																																																																																														 		 RETURNING id, name, parent_id, created_at, updated_at`,
		input.Name, input.ParentID, id).
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

	result, err := db.Pool.Exec(context.Background(),
		`DELETE FROM locations WHERE id = $1`, id)
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
