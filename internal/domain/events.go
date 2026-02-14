package domain

type UserRegistered struct {
	UserID any    `json:"userId"`
	Email  string `json:"email"`
}
type UserLoggedIn struct {
	UserID any `json:"userId"`
}
type PostCreated struct {
	PostID   any    `json:"postId"`
	AuthorID any    `json:"authorId"`
	Title    string `json:"title"`
}
type PostPublished struct {
	PostID   any    `json:"postId"`
	AuthorID any    `json:"authorId"`
	Title    string `json:"title"`
	Slug     string `json:"slug"`
}
type PostUpdated struct {
	PostID any `json:"postId"`
}
type CommentCreated struct {
	CommentID any `json:"commentId"`
	PostID    any `json:"postId"`
	AuthorID  any `json:"authorId"`
}
