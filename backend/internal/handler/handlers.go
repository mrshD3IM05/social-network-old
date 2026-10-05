package handler

import (
	"sn-backend/internal/handler/authhandler"
	"sn-backend/internal/handler/commenthandler"
	"sn-backend/internal/handler/filehandler"
	"sn-backend/internal/handler/grouphandler"
	"sn-backend/internal/handler/messagehandler"
	"sn-backend/internal/handler/notificationhandler"
	"sn-backend/internal/handler/posthandler"
	"sn-backend/internal/handler/reactionhandler"
	"sn-backend/internal/handler/requesthandler"
	"sn-backend/internal/handler/userhandler"
	"sn-backend/internal/repository"
	"sn-backend/internal/service/authsvc"
	"sn-backend/internal/service/commentsvc"
	"sn-backend/internal/service/eventsvc"
	"sn-backend/internal/service/filesvc"
	"sn-backend/internal/service/followsvc"
	"sn-backend/internal/service/groupsvc"
	"sn-backend/internal/service/messagesvc"
	"sn-backend/internal/service/notificationsvc"
	"sn-backend/internal/service/postsvc"
	"sn-backend/internal/service/reactionsvc"
	"sn-backend/internal/service/sessionsvc"
	"sn-backend/internal/service/usersvc"
	ws "sn-backend/internal/websocket"
)

type Handlers struct {
	Auth         *authhandler.Handler
	User         *userhandler.Handler
	Post         *posthandler.Handler
	Reaction     *reactionhandler.Handler
	Comment      *commenthandler.Handler
	File         *filehandler.Handler
	Group        *grouphandler.Handler
	Message      *messagehandler.Handler
	Notification *notificationhandler.Handler
	Request      *requesthandler.Handler
	WebSocket    *ws.Hub
}

// New builds every handler once, so RegisterRoutes only has to wire paths to
// methods. Each service is created a single time and shared.
func New(repos *repository.Repositories) *Handlers {
	session := sessionsvc.New(repos.Sessions)
	postService := postsvc.New(repos.Posts, repos.Follows, repos.Groups)
	fileService := filesvc.New(repos.Files, repos.Posts, repos.Comments, repos.Messages, repos.Users, "uploads")
	webSocket := ws.NewHub(repos.Messages, repos.Groups, session)
	notificationService := notificationsvc.New(repos.Notifications, webSocket)
	followService := followsvc.New(repos.Follows, repos.Users, notificationService)
	groupService := groupsvc.New(repos.Groups, repos.Users, repos.Events, repos.Messages, notificationService)
	return &Handlers{
		Auth:         authhandler.New(authsvc.New(repos.Users), session, webSocket),
		User:         userhandler.New(usersvc.New(repos.Users, repos.Follows), session, followService, postService),
		Post:         posthandler.New(postService, postsvc.NewViewerService(repos.Posts), session),
		Reaction:     reactionhandler.New(reactionsvc.New(repos.Reactions, repos.Posts), session),
		Comment:      commenthandler.New(commentsvc.New(repos.Comments, repos.Posts, notificationService), session),
		File:         filehandler.New(fileService, session),
		Group:        grouphandler.New(groupService, eventsvc.New(repos.Events, repos.Groups, repos.Users, notificationService), fileService, session),
		Message:      messagehandler.New(messagesvc.New(repos.Messages), fileService, session, webSocket),
		Request:      requesthandler.New(followService, groupService, session),
		Notification: notificationhandler.New(notificationService, session),
		WebSocket:    webSocket,
	}
}
