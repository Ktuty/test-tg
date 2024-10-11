package main

import (
	"encoding/json"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"time"
)

type Task struct {
	UserID       int64     `json:"user_id"`
	Content      string    `json:"content"`
	ReminderTime time.Time `json:"reminder_time"`
}

type User struct {
	UserID   int64  `json:"user_id"`
	ChatID   int64  `json:"chat_id"`
	Username string `json:"username"`
}

func main() {
	db, err := connectToDB()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/save_task", func(w http.ResponseWriter, r *http.Request) {
		var task Task
		err := json.NewDecoder(r.Body).Decode(&task)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_, err = db.Exec("INSERT INTO tasks (user_id, content, reminder_time) VALUES ($1, $2, $3)", task.UserID, task.Content, task.ReminderTime)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/get_tasks", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Queryx("SELECT user_id, content, reminder_time FROM tasks WHERE reminder_time <= NOW()")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var tasks []Task
		for rows.Next() {
			var task Task
			err := rows.Scan(&task.UserID, &task.Content, &task.ReminderTime)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tasks = append(tasks, task)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
	})

	http.HandleFunc("/save_user", func(w http.ResponseWriter, r *http.Request) {
		var user User
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var userID int
		err = db.QueryRow("INSERT INTO users (chat_id, username) VALUES ($1, $2) RETURNING user_id", user.ChatID, user.Username).Scan(&userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"user_id": userID})
	})

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func connectToDB() (*sqlx.DB, error) {
	connStr := "host=localhost port=5436 user=postgres password=secret dbname=postgres sslmode=disable "
	db, err := sqlx.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}
