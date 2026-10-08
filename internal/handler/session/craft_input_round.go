package session

import (
	"context"
	"io"
	"net/http"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/gin-gonic/gin"
)

// CraftInputAPI is the narrow T01 service seam. T20 owns construction and
// registration through the guarded Craft feature route table.
type CraftInputAPI interface {
	AcceptInputRound(context.Context, craft.Scope, []service.CraftInputUpload) ([]craft.Input, error)
	DecideInput(context.Context, craft.Scope, string, string) error
}

type CraftInputHandler struct{ svc CraftInputAPI }

func NewCraftInputHandler(svc CraftInputAPI) *CraftInputHandler { return &CraftInputHandler{svc: svc} }

func RegisterCraftInputRoutes(group CraftRouteGroup, handler *CraftInputHandler) {
	if group == nil || handler == nil {
		return
	}
	group.POST("/:session_id/craft/input-rounds", handler.PostInputRound)
	group.POST("/:session_id/craft/inputs/decision", handler.PostInputDecision)
}

// PostInputRound accepts a bounded multipart upload. The request is capped
// before parsing and the service validates count, actual bytes, names and
// digests again before storing a single object.
func (h *CraftInputHandler) PostInputRound(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, craft.MaxTotalInputBytes+(2<<20))
	if err := c.Request.ParseMultipartForm(1 << 20); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	defer c.Request.MultipartForm.RemoveAll()
	files := c.Request.MultipartForm.File["files"]
	digests := c.Request.MultipartForm.Value["sha256"]
	if len(files) == 0 || len(files) != len(digests) || len(files) > craft.MaxInputsPerRound {
		c.Status(http.StatusBadRequest)
		return
	}
	uploads := make([]service.CraftInputUpload, 0, len(files))
	for i, header := range files {
		if err := craft.ValidateInputName(header.Filename); err != nil {
			craftHTTPError(c, err)
			return
		}
		if header.Size <= 0 || header.Size > craft.MaxInputBytes {
			c.Status(http.StatusBadRequest)
			return
		}
		f, err := header.Open()
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		content, readErr := io.ReadAll(io.LimitReader(f, craft.MaxInputBytes+1))
		_ = f.Close()
		if readErr != nil || len(content) == 0 || len(content) > craft.MaxInputBytes {
			c.Status(http.StatusBadRequest)
			return
		}
		uploads = append(uploads, service.CraftInputUpload{Name: header.Filename, Content: content, SHA256: digests[i]})
	}
	inputs, err := h.svc.AcceptInputRound(c.Request.Context(), scope, uploads)
	if err != nil {
		craftHTTPError(c, err)
		return
	}
	data := make([]gin.H, 0, len(inputs))
	for _, in := range inputs {
		data = append(data, craftInputDTO(in))
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": data})
}

func (h *CraftInputHandler) PostInputDecision(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	var body struct {
		Ref    string `json:"ref"`
		Action string `json:"action"`
	}
	if !decodeCraftBody(c, &body) {
		return
	}
	if err := h.svc.DecideInput(c.Request.Context(), scope, body.Ref, body.Action); err != nil {
		craftHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"ref": body.Ref, "action": body.Action}})
}
