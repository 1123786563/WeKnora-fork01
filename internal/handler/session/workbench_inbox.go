package session

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

/**
 * 收件箱读模型（MX-021，B 类 GET /workbench/inbox）。
 * 通知只是提示投影：不含审批正文/凭据，点击后由客户端恢复身份→确认空间→重查权限。
 * 设备/账户隔离：注册按 (tenant, user, device) 唯一；登出即撤销该设备全部注册。
 */

// InboxNotificationRow 映射 workbench_notifications 表（只读投影列）。
type InboxNotificationRow struct {
	TenantID  uint64
	ID        string
	OwnerID   string
	Kind      string
	Title     string
	Body      string
	DeepLink  string
	Read      bool
	CreatedAt time.Time
}

func (InboxNotificationRow) TableName() string { return "workbench_notifications" }

// DeviceRegistrationRow 映射 workbench_device_registrations（推送 token 注册）。
type DeviceRegistrationRow struct {
	TenantID  uint64
	DeviceID  string
	OwnerID   string
	Token     string
	Platform  string
	Revoked   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (DeviceRegistrationRow) TableName() string { return "workbench_device_registrations" }

type InboxCounts struct {
	Unread int64 `json:"unread_count"`
}

type InboxItem struct {
	NotificationID string `json:"notification_id"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	CreatedAt      string `json:"created_at"`
	Read           bool   `json:"read"`
	DeepLink       string `json:"deep_link,omitempty"`
}

type InboxPage struct {
	Items      []InboxItem `json:"items"`
	Unread     int64       `json:"unread_count"`
	NextCursor string      `json:"next_cursor"`
}

const inboxPageLimit = 50

type InboxService struct {
	db    *gorm.DB
	clock func() time.Time
}

func NewWorkbenchInboxService(db *gorm.DB, clock func() time.Time) *InboxService {
	if clock == nil {
		clock = time.Now
	}
	return &InboxService{db: db, clock: clock}
}

// Inbox 以 tenant+owner 谓词读取；通知只提示（无审批正文授权语义）。
func (s *InboxService) Inbox(ctx context.Context, tenantID uint64, ownerID string, cursor string) (InboxPage, error) {
	if tenantID == 0 || ownerID == "" {
		return InboxPage{}, errors.New("identity required")
	}
	query := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ?", tenantID, ownerID)
	if cursor != "" {
		query = query.Where("created_at < ?", cursor)
	}
	var rows []InboxNotificationRow
	if err := query.Order("created_at DESC").Limit(inboxPageLimit + 1).Find(&rows).Error; err != nil {
		return InboxPage{}, err
	}
	next := ""
	if len(rows) > inboxPageLimit {
		rows = rows[:inboxPageLimit]
		next = rows[len(rows)-1].CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	items := make([]InboxItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, InboxItem{
			NotificationID: row.ID,
			Kind:           row.Kind,
			Title:          row.Title,
			Body:           row.Body,
			CreatedAt:      row.CreatedAt.UTC().Format(time.RFC3339),
			Read:           row.Read,
			DeepLink:       row.DeepLink,
		})
	}
	var unread int64
	if err := s.db.WithContext(ctx).Model(&InboxNotificationRow{}).
		Where("tenant_id = ? AND owner_id = ? AND read = ?", tenantID, ownerID, false).
		Count(&unread).Error; err != nil {
		return InboxPage{}, err
	}
	return InboxPage{Items: items, Unread: unread, NextCursor: next}, nil
}

// MarkRead 幂等标记已读：通知已读不执行通知描述的任何操作。
func (s *InboxService) MarkRead(ctx context.Context, tenantID uint64, ownerID, notificationID string) error {
	if tenantID == 0 || ownerID == "" || notificationID == "" {
		return errors.New("identity and notification required")
	}
	return s.db.WithContext(ctx).Model(&InboxNotificationRow{}).
		Where("tenant_id = ? AND owner_id = ? AND id = ?", tenantID, ownerID, notificationID).
		Update("read", true).Error
}

// RegisterDevice 注册推送：按 (tenant, owner, device) upsert；token 变更换新。
func (s *InboxService) RegisterDevice(ctx context.Context, tenantID uint64, ownerID, deviceID, token, platform string) error {
	if tenantID == 0 || ownerID == "" || deviceID == "" || token == "" {
		return errors.New("identity, device and token required")
	}
	var existing DeviceRegistrationRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND owner_id = ? AND device_id = ?", tenantID, ownerID, deviceID).
		Take(&existing).Error
	now := s.clock()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(&DeviceRegistrationRow{
			TenantID: tenantID, DeviceID: deviceID, OwnerID: ownerID,
			Token: token, Platform: platform, CreatedAt: now, UpdatedAt: now,
		}).Error
	}
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&DeviceRegistrationRow{}).
		Where("tenant_id = ? AND owner_id = ? AND device_id = ?", tenantID, ownerID, deviceID).
		Updates(map[string]any{"token": token, "revoked": false, "updated_at": now}).Error
}

// RevokeDevicesForOwner 登出撤销：该用户全部设备注册失效（跨账户通知不投递）。
func (s *InboxService) RevokeDevicesForOwner(ctx context.Context, tenantID uint64, ownerID string) error {
	return s.db.WithContext(ctx).Model(&DeviceRegistrationRow{}).
		Where("tenant_id = ? AND owner_id = ?", tenantID, ownerID).
		Update("revoked", true).Error
}

// WorkbenchInboxHandler 暴露收件箱读模型与已读/注册（B 类）。
type WorkbenchInboxHandler struct {
	inbox *InboxService
}

func NewWorkbenchInboxHandler(inbox *InboxService) *WorkbenchInboxHandler {
	return &WorkbenchInboxHandler{inbox: inbox}
}

// inboxIdentity 与读 handler 相同的双通道身份提取（context 优先，gin 回退）。
func (h *WorkbenchInboxHandler) identity(c *gin.Context) (uint64, string, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		if value, exists := c.Get(types.TenantIDContextKey.String()); exists {
			tenantID, ok = value.(uint64)
		}
	}
	userID, userOK := types.UserIDFromContext(c.Request.Context())
	if !userOK || userID == "" {
		if value, exists := c.Get(types.UserIDContextKey.String()); exists {
			userID, userOK = value.(string)
		}
	}
	if !ok || !userOK || tenantID == 0 || userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "identity required"})
		return 0, "", false
	}
	return tenantID, userID, true
}

func (h *WorkbenchInboxHandler) Inbox(c *gin.Context) {
	if h == nil || h.inbox == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	tenantID, userID, ok := h.identity(c)
	if !ok {
		return
	}
	page, err := h.inbox.Inbox(c.Request.Context(), tenantID, userID, c.Query("cursor"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": page})
}

type inboxReadRequest struct {
	NotificationID string `json:"notification_id" binding:"required"`
}

func (h *WorkbenchInboxHandler) MarkRead(c *gin.Context) {
	if h == nil || h.inbox == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	tenantID, userID, ok := h.identity(c)
	if !ok {
		return
	}
	var input inboxReadRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "notification_id required"})
		return
	}
	if err := h.inbox.MarkRead(c.Request.Context(), tenantID, userID, input.NotificationID); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"notification_id": input.NotificationID, "read": true}})
}

type deviceRegisterRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform"`
}

func (h *WorkbenchInboxHandler) RegisterDevice(c *gin.Context) {
	if h == nil || h.inbox == nil {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	tenantID, userID, ok := h.identity(c)
	if !ok {
		return
	}
	var input deviceRegisterRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "device_id and token required"})
		return
	}
	if err := h.inbox.RegisterDevice(c.Request.Context(), tenantID, userID, input.DeviceID, input.Token, input.Platform); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"device_id": input.DeviceID, "registered": true}})
}
