package main

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/slok/go-http-metrics/middleware"
)

//same as sloks gin middleware except uses fullpath for url path
//https://github.com/slok/go-http-metrics/pull/93
func promHandler(m middleware.Middleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		r := &reporter{c: c}
		m.Measure("", r, func() {
			c.Next()
		})
	}
}

type reporter struct {
	c *gin.Context
}

func (r *reporter) Method() string { return r.c.Request.Method }

func (r *reporter) Context() context.Context { return r.c.Request.Context() }

func (r *reporter) URLPath() string { return r.c.FullPath() }

func (r *reporter) StatusCode() int { return r.c.Writer.Status() }

func (r *reporter) BytesWritten() int64 { return int64(r.c.Writer.Size()) }
