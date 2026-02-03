package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type PantryItem struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Quantity  *string   `json:"quantity"` // pointer so it can be null
	CreatedAt time.Time `json:"created_at"`
}

type CreatePantryItemRequest struct {
	UserID   string  `json:"user_id"`            // for MVP: "demo_user"
	Name     string  `json:"name"`               // required
	Quantity *string `json:"quantity,omitempty"` // optional
}

func main() {
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is missing in environment")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create Postgres connection pool
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("failed to create db pool: %v", err)
	}
	defer pool.Close()

	// Verify DB connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	r := gin.Default()

	// Base route (optional nice-to-have)
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "PantryToPlate API running"})
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// DB test
	r.GET("/db-test", func(c *gin.Context) {
		var now time.Time
		err := pool.QueryRow(context.Background(), "select now()").Scan(&now)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "db query failed", "details": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"db_time": now})
	})

	// -------------------------
	// Pantry CRUD (Step 3)
	// -------------------------

	// CREATE: Add item to pantry
	r.POST("/pantry/items", func(c *gin.Context) {
		var req CreatePantryItemRequest

		// Parse JSON body into req
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body", "details": err.Error()})
			return
		}

		// Basic validation
		if req.UserID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required (use 'demo_user' for MVP)"})
			return
		}
		if req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
			return
		}

		// Insert into DB and return the created row
		var item PantryItem
		insertSQL := `
			insert into public.pantry_items (user_id, name, quantity)
			values ($1, $2, $3)
			returning id, user_id, name, quantity, created_at;
		`

		err := pool.QueryRow(
			context.Background(),
			insertSQL,
			req.UserID,
			req.Name,
			req.Quantity,
		).Scan(&item.ID, &item.UserID, &item.Name, &item.Quantity, &item.CreatedAt)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert pantry item", "details": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, item)
	})

	// READ: List all pantry items for a user
	// Usage: /pantry/items?user_id=demo_user
	r.GET("/pantry/items", func(c *gin.Context) {
		userID := c.Query("user_id")
		if userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id query param is required (example: ?user_id=demo_user)"})
			return
		}

		querySQL := `
			select id, user_id, name, quantity, created_at
			from public.pantry_items
			where user_id = $1
			order by created_at desc;
		`

		rows, err := pool.Query(context.Background(), querySQL, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query pantry items", "details": err.Error()})
			return
		}
		defer rows.Close()

		items := make([]PantryItem, 0)
		for rows.Next() {
			var item PantryItem
			if err := rows.Scan(&item.ID, &item.UserID, &item.Name, &item.Quantity, &item.CreatedAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read row", "details": err.Error()})
				return
			}
			items = append(items, item)
		}

		c.JSON(http.StatusOK, gin.H{"items": items})
	})

	// DELETE: Delete pantry item by id
	r.DELETE("/pantry/items/:id", func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
			return
		}

		deleteSQL := `delete from public.pantry_items where id = $1;`
		cmdTag, err := pool.Exec(context.Background(), deleteSQL, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete pantry item", "details": err.Error()})
			return
		}

		// cmdTag.RowsAffected() tells how many rows were deleted (0 means id not found)
		if cmdTag.RowsAffected() == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"deleted": true, "id": id})
	})

	log.Printf("server running on http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
