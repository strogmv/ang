package api

import "github.com/strogmv/ang/cue/schema"

HTTP: schema.#HTTP & {
	// Auth
	Register: {
		method: "POST"
		path:   "/auth/register"
	}
	Login: {
		method: "POST"
		path:   "/auth/login"
	}
	GetProfile: {
		method: "GET"
		path:   "/auth/profile"
		auth: {
			type:   "jwt"
			inject: "userId"
		}
	}
	UpdateProfile: {
		method: "PUT"
		path:   "/auth/profile"
		auth: {
			type:   "jwt"
			inject: "userId"
		}
	}

	// Tags
	ListTags: {
		method: "GET"
		path:   "/tags"
	}
	CreateTag: {
		method: "POST"
		path:   "/tags"
		auth: {
			type:  "jwt"
			roles: ["admin"]
		}
	}
	UpdateTag: {
		method: "PUT"
		path:   "/tags/{id}"
		auth: {
			type:  "jwt"
			roles: ["admin"]
		}
	}
	DeleteTag: {
		method: "DELETE"
		path:   "/tags/{id}"
		auth: {
			type:  "jwt"
			roles: ["admin"]
		}
	}

	// Posts
	CreatePost: {
		method: "POST"
		path:   "/posts"
		auth: {
			type:   "jwt"
			roles:  ["author", "admin"]
			inject: "userId"
		}
	}
	GetPost: {
		method: "GET"
		path:   "/posts/{slug}"
	}
	ListPosts: {
		method: "GET"
		path:   "/posts"
	}
	ListMyPosts: {
		method: "GET"
		path:   "/my/posts"
		auth: {
			type:   "jwt"
			roles:  ["author", "admin"]
			inject: "userId"
		}
	}
	UpdatePost: {
		method: "PUT"
		path:   "/posts/{id}"
		auth: {
			type:  "jwt"
			roles: ["author", "admin"]
		}
	}
	SubmitPost: {
		method: "POST"
		path:   "/posts/{id}/submit"
		auth: {
			type:  "jwt"
			roles: ["author", "admin"]
		}
	}
	PublishPost: {
		method: "POST"
		path:   "/posts/{id}/publish"
		auth: {
			type:  "jwt"
			roles: ["admin"]
		}
	}
	ArchivePost: {
		method: "POST"
		path:   "/posts/{id}/archive"
		auth: {
			type:  "jwt"
			roles: ["author", "admin"]
		}
	}
	DeletePost: {
		method: "DELETE"
		path:   "/posts/{id}"
		auth: {
			type:  "jwt"
			roles: ["author", "admin"]
		}
	}

	// Comments
	CreateComment: {
		method: "POST"
		path:   "/posts/{postID}/comments"
		auth: {
			type:   "jwt"
			inject: "userId"
		}
	}
	ListComments: {
		method: "GET"
		path:   "/posts/{postID}/comments"
	}
	UpdateComment: {
		method: "PUT"
		path:   "/comments/{id}"
		auth: {
			type:   "jwt"
			inject: "userId"
		}
	}
	DeleteComment: {
		method: "DELETE"
		path:   "/comments/{id}"
		auth: {
			type:   "jwt"
			inject: "userId"
		}
	}
}
