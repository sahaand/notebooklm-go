package notebooklm

import (
	"context"
	"fmt"

	"github.com/saeedata/notebooklm-go/rpc"
)

// SharingAPI provides notebook sharing management operations.
type SharingAPI struct {
	client *Client
}

// GetStatus returns the current sharing configuration for a notebook.
func (s *SharingAPI) GetStatus(ctx context.Context, notebookID string) (*ShareStatus, error) {
	params := []any{notebookID}
	result, err := s.client.RPCCall(ctx, rpc.GetShareStatus, params, "/notebook/"+notebookID, false)
	if err != nil {
		return nil, fmt.Errorf("get share status for notebook %s: %w", notebookID, err)
	}

	return parseShareStatus(result, notebookID), nil
}

// SetPublic enables or disables public link sharing.
func (s *SharingAPI) SetPublic(ctx context.Context, notebookID string, public bool) (*ShareStatus, error) {
	accessCode := int(rpc.ShareRestricted)
	if public {
		accessCode = int(rpc.ShareAnyoneWithLink)
	}

	params := []any{notebookID, accessCode}
	_, err := s.client.RPCCall(ctx, rpc.ShareNotebook, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("set public sharing: %w", err)
	}

	return s.GetStatus(ctx, notebookID)
}

// SetViewLevel configures what shared viewers can access.
func (s *SharingAPI) SetViewLevel(ctx context.Context, notebookID string, level rpc.ShareViewLevel) (*ShareStatus, error) {
	params := []any{notebookID, nil, int(level)}
	_, err := s.client.RPCCall(ctx, rpc.ShareNotebook, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("set view level: %w", err)
	}

	status, err := s.GetStatus(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	// Override since the GET_SHARE_STATUS API may not return view_level immediately
	status.ViewLevel = level
	return status, nil
}

// AddUser shares a notebook with a specific user by email.
func (s *SharingAPI) AddUser(ctx context.Context, notebookID, email string, permission rpc.SharePermission, sendNotification bool, message string) (*ShareStatus, error) {
	if permission == rpc.ShareOwner {
		return nil, fmt.Errorf("cannot assign OWNER permission")
	}
	if permission == rpc.ShareRemove {
		return nil, fmt.Errorf("use RemoveUser to remove users")
	}

	params := []any{notebookID, []any{[]any{email, int(permission)}}, sendNotification, message}
	_, err := s.client.RPCCall(ctx, rpc.ShareNotebook, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("add user %s: %w", email, err)
	}

	return s.GetStatus(ctx, notebookID)
}

// UpdateUser modifies an existing user's permission level.
func (s *SharingAPI) UpdateUser(ctx context.Context, notebookID, email string, permission rpc.SharePermission) (*ShareStatus, error) {
	params := []any{notebookID, []any{[]any{email, int(permission)}}, false, ""}
	_, err := s.client.RPCCall(ctx, rpc.ShareNotebook, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("update user %s: %w", email, err)
	}
	return s.GetStatus(ctx, notebookID)
}

// RemoveUser revokes a user's access to the notebook.
func (s *SharingAPI) RemoveUser(ctx context.Context, notebookID, email string) (*ShareStatus, error) {
	params := []any{notebookID, []any{[]any{email, int(rpc.ShareRemove)}}, false, ""}
	_, err := s.client.RPCCall(ctx, rpc.ShareNotebook, params, "/notebook/"+notebookID, true)
	if err != nil {
		return nil, fmt.Errorf("remove user %s: %w", email, err)
	}
	return s.GetStatus(ctx, notebookID)
}

func parseShareStatus(result any, notebookID string) *ShareStatus {
	status := &ShareStatus{NotebookID: notebookID}

	arr, _ := result.([]any)
	if len(arr) == 0 {
		return status
	}

	// Parse access level
	if len(arr) > 0 {
		if code, ok := arr[0].(float64); ok {
			status.Access = rpc.ShareAccess(int(code))
			status.IsPublic = status.Access == rpc.ShareAnyoneWithLink
		}
	}

	// Build share URL if public
	if status.IsPublic {
		status.ShareURL = fmt.Sprintf("https://notebooklm.google.com/notebook/%s", notebookID)
	}

	// Parse shared users if present
	if len(arr) > 2 {
		if users, ok := arr[2].([]any); ok {
			for _, u := range users {
				if userArr, ok := u.([]any); ok && len(userArr) >= 2 {
					email, _ := userArr[0].(string)
					permCode, _ := userArr[1].(float64)
					user := SharedUser{
						Email:      email,
						Permission: rpc.SharePermission(int(permCode)),
					}
					if len(userArr) > 2 {
						user.DisplayName, _ = userArr[2].(string)
					}
					status.SharedUsers = append(status.SharedUsers, user)
				}
			}
		}
	}

	return status
}
