package routers

import (
	"io"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddleware(t *testing.T) {
	// mid1 pre
	// mid2 pre
	// mid3 pre
	// mid3 post
	// mid2 post
	// mid1 post

	mid1 := func(c *gin.Context) {
		t.Log("mid1 pre")
		c.Next()
		t.Log("mid1 post")
	}

	mid2 := func(c *gin.Context) {
		t.Log("mid2 pre")
		c.Next()
		t.Log("mid2 post")
	}

	mid3 := func(c *gin.Context) {
		t.Log("mid3 pre")
		c.Next()
		t.Log("mid3 post")
	}

	r := gin.Default()
	r.Use(mid1, mid2, mid3)
	r.GET("/", func(ctx *gin.Context) {
		_, _ = io.WriteString(ctx.Writer, "hello")
	})

	// _ = r.Run(":8080")
}
