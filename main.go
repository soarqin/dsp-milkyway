package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func main() {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	stor, err := NewStorage()
	if err != nil {
		log.Fatal(err)
	}
	r := gin.Default()
	r.GET("/login/header", func(c *gin.Context) {
		userId := c.Query("user_id")
		key := rnd.Uint32()
		stor.AssignLoginKey(userId, key)
		c.String(http.StatusOK, "%v,%v", key, stor.GetLastRankUrl())
	})
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
	stor.Close()
}
