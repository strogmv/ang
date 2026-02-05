# Blog Example

A comprehensive ANG example demonstrating a blog platform with authentication, posts, comments, and role-based access control.

## Features Demonstrated

### Domain Modeling (`cue/domain/entities.cue`)
- **Entities**: User, Post, Comment, Tag, PostTag
- **Relationships**: Foreign keys, many-to-many (posts ↔ tags)
- **Validation**: Email, min/max length, required fields
- **Timestamps**: createdAt, updatedAt auto-management

### State Machine (`cue/domain/entities.cue`)
Post lifecycle with FSM:
```
draft → review → published → archived
         ↓                      ↓
       draft ←─────────────────┘
```

### Flow DSL Patterns (`cue/api/*.cue`)

1. **Create with validation and events**
   - `CreatePost`: Create entity, handle tags, publish event

2. **Read with ownership checks**
   - `GetPost`: Find, verify status, increment counter

3. **Update with conditional logic**
   - `UpdatePost`: Ownership check, field-by-field update

4. **FSM transitions**
   - `SubmitForReview`: draft → review
   - `PublishPost`: review → published
   - `ArchivePost`: published → archived

5. **Transactions**
   - `DeletePost`: Delete related records in transaction

6. **Nested resources**
   - `CreateComment`: Verify parent exists, handle replies

### Repository Finders (`cue/repo/repositories.cue`)
- Simple filters: `FindByEmail`, `FindBySlug`
- Pagination: `ListPublished`, `ListByAuthor`
- Counts: `CountPublished`, `CountByPost`
- Subqueries: `ListPublishedByTag`, `ListByPost`
- Delete actions: `DeleteByPost`, `DeleteByParent`

### Authentication & Authorization
- JWT-based auth with token injection
- Role-based access: `reader`, `author`, `admin`
- Ownership checks: "Access denied" for non-owners

### Events (`cue/architecture/services.cue`)
- `UserRegistered`, `UserLoggedIn`
- `PostCreated`, `PostPublished`, `PostUpdated`
- `CommentCreated`

## Project Structure

```
blog/
├── ang.yaml                    # Project configuration
├── cue/
│   ├── api/
│   │   ├── auth.cue           # Auth operations (register, login, profile)
│   │   ├── posts.cue          # Post CRUD + FSM transitions
│   │   ├── comments.cue       # Comment operations
│   │   └── tags.cue           # Tag management (admin)
│   ├── domain/
│   │   └── entities.cue       # Entity definitions
│   ├── architecture/
│   │   └── services.cue       # Service definitions + events
│   ├── repo/
│   │   └── repositories.cue   # Custom repository finders
│   └── policies/
│       └── rbac.cue           # Role-based access control
└── README.md
```

## Usage

```bash
# From the blog example directory
cd examples/blog

# Validate the architecture
ang validate

# Generate code
ang build

# Generated structure:
# ├── cmd/server/main.go
# ├── internal/
# │   ├── domain/           # Entities, events
# │   ├── port/             # Service interfaces
# │   ├── service/          # Business logic
# │   ├── adapter/
# │   │   └── repository/   # PostgreSQL repos
# │   └── transport/
# │       └── http/         # HTTP handlers
# └── sdk/                  # TypeScript client
```

## API Endpoints

### Auth
- `POST /auth/register` - Register new user
- `POST /auth/login` - Login and get tokens
- `GET /auth/profile` - Get current user (auth required)
- `PUT /auth/profile` - Update profile (auth required)

### Posts
- `GET /posts` - List published posts (public)
- `GET /posts/{slug}` - Get post by slug (public)
- `POST /posts` - Create draft (author/admin)
- `PUT /posts/{id}` - Update post (author/admin)
- `DELETE /posts/{id}` - Delete draft (author/admin)
- `POST /posts/{id}/submit` - Submit for review (author)
- `POST /posts/{id}/publish` - Publish post (admin)
- `POST /posts/{id}/archive` - Archive post (author/admin)
- `GET /my/posts` - List own posts (author/admin)

### Comments
- `GET /posts/{postID}/comments` - List comments (public)
- `POST /posts/{postID}/comments` - Add comment (auth required)
- `PUT /comments/{id}` - Update own comment (auth required)
- `DELETE /comments/{id}` - Delete comment (auth required)

### Tags
- `GET /tags` - List all tags (public)
- `POST /tags` - Create tag (admin)
- `PUT /tags/{id}` - Update tag (admin)
- `DELETE /tags/{id}` - Delete tag (admin)

## Learning Path

1. Start with `cue/domain/entities.cue` to understand entity modeling
2. Study `cue/api/auth.cue` for basic CRUD with Flow DSL
3. Explore `cue/api/posts.cue` for advanced patterns (FSM, transactions)
4. Check `cue/repo/repositories.cue` for custom queries
5. Review `cue/policies/rbac.cue` for access control
