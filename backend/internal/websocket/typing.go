package websocket

// relayTyping tells the other side that somebody is writing: one person in a
// private chat, or the rest of the members in a group.
//
// Typing is not saved anywhere. It only means "right now", so it is passed on
// and forgotten. The same permission check as a real message applies, so nobody
// can ping a person or a group they may not write to.
func (h *Hub) relayTyping(fromUserID int64, toUserID, groupID *int64) {
	allowed, err := h.messages.CanMessage(fromUserID, toUserID, groupID)
	if err != nil || !allowed {
		return
	}

	event := map[string]any{"type": "typing", "from_user_id": fromUserID}

	if groupID != nil {
		event["group_id"] = *groupID
		members, err := h.groups.GroupMemberIDs(*groupID)
		if err != nil {
			return
		}
		for _, memberID := range members {
			if memberID != fromUserID {
				h.publish(memberID, event)
			}
		}
		return
	}

	if toUserID != nil {
		h.publish(*toUserID, event)
	}
}
