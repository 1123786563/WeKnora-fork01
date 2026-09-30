package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Recovery is a middleware that recovers from panics.
//
// http.ErrAbortHandler is the net/http sentinel for "abort this connection
// after bytes were already written" (the craft export bundle's mid-stream
// integrity contract panics with it once the 200 head and partial zip bytes
// have left the process). Swallowing it here would append a 500 JSON body to
// the truncated stream and terminate the chunked response cleanly, handing
// the client a "successful" download of a degraded bundle. The sentinel is
// therefore re-panicked so net/http tears the connection down, exactly as
// handlers that use it intend; every other panic still becomes a 500.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				if httpErr, ok := err.(error); ok && errors.Is(httpErr, http.ErrAbortHandler) {
					panic(err)
				}
				// Get request ID from context
				ctx := c.Request.Context()
				requestID, _ := c.Get("RequestID")

				// Print stacktrace
				stacktrace := debug.Stack()
				// Log error with structured logger
				logger.ErrorWithFields(ctx, fmt.Errorf("panic: %v", err), logrus.Fields{
					"request_id": requestID,
					"stacktrace": string(stacktrace),
				})

				// 返回500错误
				c.AbortWithStatusJSON(500, gin.H{
					"error":   "Internal Server Error",
					"message": fmt.Sprintf("%v", err),
				})
			}
		}()

		c.Next()
	}
}
