package craft

// TaskRole is authority on one private Craft Task, whose ID is SessionID.
// Tenant roles and resource permissions never widen this role.
type TaskRole string

const (
	TaskRoleOwner        TaskRole = "owner"
	TaskRoleCollaborator TaskRole = "collaborator"
	TaskRoleViewer       TaskRole = "viewer"
)

func (r TaskRole) Grantable() bool {
	return r == TaskRoleCollaborator || r == TaskRoleViewer
}

// AllowsTaskAction checks only Task membership. TaskOpenSource still needs
// the caller's own current source ACL at the original-resource resolver.
func (r TaskRole) AllowsTaskAction(action TaskAction) bool {
	switch action {
	case TaskRead, TaskPreview, TaskOpenSource:
		return r == TaskRoleOwner || r == TaskRoleCollaborator || r == TaskRoleViewer
	case TaskWrite:
		return r == TaskRoleOwner || r == TaskRoleCollaborator
	case TaskShare:
		return r == TaskRoleOwner
	default:
		return false
	}
}

type TaskMember struct {
	UserID string   `json:"user_id"`
	Role   TaskRole `json:"role"`
}
