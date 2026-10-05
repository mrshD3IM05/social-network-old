package websocket

import "sn-backend/internal/model"

// PublishMessage pushes a message to whoever should see it: both sides of a
// private chat, or every member of a group. It is used by the HTTP send
// endpoint, which is the path that can carry images.
func (h *Hub) PublishMessage(message *model.Message) {
	event := map[string]any{"type": "message", "message": message}

	if message.GroupID != nil {
		members, err := h.groups.GroupMemberIDs(*message.GroupID)
		if err != nil {
			return
		}
		for _, memberID := range members {
			h.publish(memberID, event)
		}
		return
	}

	if message.ToUserID != nil {
		h.publish(*message.ToUserID, event)
	}
	h.publish(message.FromUserID, event)
}
