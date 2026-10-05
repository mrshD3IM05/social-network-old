package server

import (
	"net/http"

	handlers "sn-backend/internal/handler"
	"sn-backend/internal/middleware"
)

func RegisterRoutes(mux *http.ServeMux, h *handlers.Handlers) {
	auth := middleware.NewAuth(h.Auth.Session)

	//Auth routes
	mux.Handle("POST /register", middleware.AuthRateLimit(auth.Guest(http.HandlerFunc(h.Auth.Register))))
	mux.Handle("POST /login", middleware.AuthRateLimit(auth.Guest(http.HandlerFunc(h.Auth.Login))))
	mux.Handle("POST /logout", auth.Authorized(http.HandlerFunc(h.Auth.Logout)))
	mux.Handle("GET /me", auth.Authorized(http.HandlerFunc(h.Auth.Me)))

	// user routes
	mux.Handle("GET /users", auth.Authorized(http.HandlerFunc(h.User.ListUsers)))
	mux.Handle("GET /user/{id}", auth.Authorized(http.HandlerFunc(h.User.GetUser)))
	mux.Handle("GET /users/{id}/posts", auth.Authorized(http.HandlerFunc(h.User.UserPosts)))
	mux.Handle("GET /users/{id}/followers", auth.Authorized(http.HandlerFunc(h.User.Followers)))
	mux.Handle("GET /users/{id}/following", auth.Authorized(http.HandlerFunc(h.User.Following)))
	mux.Handle("GET /users/suggestions", auth.Authorized(http.HandlerFunc(h.User.Suggestions)))
	mux.Handle("GET /contacts", auth.Authorized(http.HandlerFunc(h.User.Contacts)))
	mux.Handle("POST /users/{id}/follow", auth.Authorized(http.HandlerFunc(h.User.FollowUser)))
	mux.Handle("DELETE /users/{id}/follow", auth.Authorized(http.HandlerFunc(h.User.UnfollowUser)))
	mux.Handle("POST /follow-requests/{id}/accept", auth.Authorized(http.HandlerFunc(h.User.RespondFollow)))
	mux.Handle("POST /follow-requests/{id}/decline", auth.Authorized(http.HandlerFunc(h.User.RespondFollow)))
	mux.Handle("PUT /me/privacy", auth.Authorized(http.HandlerFunc(h.User.SetPrivacy)))

	// everything waiting for the caller to accept or decline, in one response
	mux.Handle("GET /requests", auth.Authorized(http.HandlerFunc(h.Request.Pending)))

	// notification routes
	mux.Handle("GET /notifications", auth.Authorized(http.HandlerFunc(h.Notification.List)))
	mux.Handle("GET /notifications/unread", auth.Authorized(http.HandlerFunc(h.Notification.UnreadCount)))
	mux.Handle("POST /notifications/read", auth.Authorized(http.HandlerFunc(h.Notification.MarkRead)))

	// post routes
	mux.Handle("GET /posts", auth.Authorized(http.HandlerFunc(h.Post.ListPosts)))
	mux.Handle("POST /posts", auth.Authorized(http.HandlerFunc(h.Post.CreatePost)))
	mux.Handle("PUT /posts/{id}", auth.Authorized(http.HandlerFunc(h.Post.UpdatePost)))
	mux.Handle("DELETE /posts/{id}", auth.Authorized(http.HandlerFunc(h.Post.DeletePost)))
	mux.Handle("GET /posts/{id}", auth.Authorized(http.HandlerFunc(h.Post.GetPost)))

	// comment routes (visibility follows the post: privacy rules or group membership)
	mux.Handle("GET /posts/{id}/viewers", auth.Authorized(http.HandlerFunc(h.Post.ListViewers)))
	mux.Handle("GET /posts/{id}/comments", auth.Authorized(http.HandlerFunc(h.Comment.ListComments)))
	mux.Handle("POST /posts/{id}/comments", auth.Authorized(http.HandlerFunc(h.Comment.CreateComment)))
	mux.Handle("PUT /comments/{id}", auth.Authorized(http.HandlerFunc(h.Comment.UpdateComment)))
	mux.Handle("DELETE /comments/{id}", auth.Authorized(http.HandlerFunc(h.Comment.DeleteComment)))

	// reaction routes
	mux.Handle("POST /posts/{id}/reactions", auth.Authorized(http.HandlerFunc(h.Reaction.CreateReaction)))
	mux.Handle("DELETE /posts/{id}/reactions", auth.Authorized(http.HandlerFunc(h.Reaction.DeleteReaction)))

	// file routes
	mux.Handle("POST /avatar", auth.Authorized(http.HandlerFunc(h.File.SetAvatar)))
	mux.Handle("GET /fs/{id}", auth.Authorized(http.HandlerFunc(h.File.Download)))

	// group routes
	mux.Handle("POST /groups", auth.Authorized(http.HandlerFunc(h.Group.CreateGroup)))
	mux.Handle("GET /groups", auth.Authorized(http.HandlerFunc(h.Group.ListGroups)))
	mux.Handle("GET /groups/{group_id}", auth.Authorized(http.HandlerFunc(h.Group.GetGroup)))
	mux.Handle("PUT /groups/{group_id}", auth.Authorized(http.HandlerFunc(h.Group.UpdateGroup)))
	mux.Handle("DELETE /groups/{group_id}", auth.Authorized(http.HandlerFunc(h.Group.DeleteGroup)))
	mux.Handle("POST /groups/{group_id}/avatar", auth.Authorized(http.HandlerFunc(h.Group.SetGroupAvatar)))
	mux.Handle("GET /groups/{group_id}/members", auth.Authorized(http.HandlerFunc(h.Group.GetGroupMembers)))
	mux.Handle("DELETE /groups/{group_id}/members/{userID}", auth.Authorized(http.HandlerFunc(h.Group.RemoveMember)))
	mux.Handle("GET /groups/{group_id}/messages", auth.Authorized(http.HandlerFunc(h.Group.ListMessages)))
	mux.Handle("POST /groups/{group_id}/invitations", auth.Authorized(http.HandlerFunc(h.Group.InviteUser)))
	mux.Handle("POST /group-invitations/{id}/accept", auth.Authorized(http.HandlerFunc(h.Group.RespondInvitation)))
	mux.Handle("POST /group-invitations/{id}/decline", auth.Authorized(http.HandlerFunc(h.Group.RespondInvitation)))
	mux.Handle("GET /group-invitations", auth.Authorized(http.HandlerFunc(h.Group.PendingInvitations)))
	mux.Handle("POST /groups/{group_id}/join-requests", auth.Authorized(http.HandlerFunc(h.Group.RequestJoin)))
	mux.Handle("POST /group-join-requests/{id}/accept", auth.Authorized(http.HandlerFunc(h.Group.RespondJoinRequest)))
	mux.Handle("POST /group-join-requests/{id}/decline", auth.Authorized(http.HandlerFunc(h.Group.RespondJoinRequest)))
	mux.Handle("GET /groups/{group_id}/join-requests", auth.Authorized(http.HandlerFunc(h.Group.PendingJoinRequests)))

	// group post routes (members only, enforced in the services)
	mux.Handle("GET /groups/{group_id}/posts", auth.Authorized(http.HandlerFunc(h.Post.ListPosts)))
	mux.Handle("POST /groups/{group_id}/posts", auth.Authorized(http.HandlerFunc(h.Post.CreatePost)))
	mux.Handle("PUT /groups/{group_id}/posts/{post_id}", auth.Authorized(http.HandlerFunc(h.Post.UpdatePost)))
	mux.Handle("DELETE /groups/{group_id}/posts/{post_id}", auth.Authorized(http.HandlerFunc(h.Post.DeletePost)))

	// group event routes (members only, enforced in the services)
	mux.Handle("GET /groups/{group_id}/events", auth.Authorized(http.HandlerFunc(h.Group.ListEvents)))
	mux.Handle("POST /groups/{group_id}/events", auth.Authorized(http.HandlerFunc(h.Group.CreateEvent)))
	mux.Handle("GET /events/upcoming", auth.Authorized(http.HandlerFunc(h.Group.UpcomingEvents)))
	mux.Handle("POST /events/{id}/response", auth.Authorized(http.HandlerFunc(h.Group.RespondEvent)))

	// direct message routes (the sender must follow, or be followed by, the recipient)
	mux.Handle("GET /messages/{id}", auth.Authorized(http.HandlerFunc(h.Message.History)))
	mux.Handle("POST /messages", auth.Authorized(http.HandlerFunc(h.Message.Send)))
	mux.Handle("POST /messages/{id}/images", auth.Authorized(http.HandlerFunc(h.Message.AttachImages)))

	// websocket routes
	mux.Handle("GET /ws", auth.Authorized(h.WebSocket))
}
