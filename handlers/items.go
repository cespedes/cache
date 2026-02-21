package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cespedes/cache/db"
	"github.com/cespedes/cache/models"
)

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
	query += ` ORDER BY name`

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
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name == "" || input.LocationID == 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var item models.Item
	err := db.Pool.QueryRow(context.Background(),
		`INSERT INTO items (name, location_id) VALUES ($1, $2)
		RETURNING id, name, location_id, created_at, updated_at`,
		input.Name, input.LocationID).
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
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Name == "" || input.LocationID == 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	var item models.Item
	err = db.Pool.QueryRow(context.Background(),
		`UPDATE items SET name = $1, location_id = $2
		WHERE id = $3
		RETURNING id, name, location_id, created_at, updated_at`,
		input.Name, input.LocationID, id).
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
