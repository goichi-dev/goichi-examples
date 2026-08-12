package internal

// LoginRequest is the request body for the /login endpoint
type LoginRequest struct {
	Username string `json:"username" desc:"The user's username"`
	Password string `json:"password" desc:"The user's password"`
}

// LoginResponse is the successful response for the /login endpoint
type LoginResponse struct {
	Token string `json:"token" desc:"JWT authentication token"`
}

// ErrorResponse is a generic error response
type ErrorResponse struct {
	Error string `json:"error" desc:"Error message"`
}

// Todo is the resource used by the REST example
type Todo struct {
	ID    int    `json:"id" desc:"Todo identifier"`
	Title string `json:"title" desc:"What needs doing"`
	Done  bool   `json:"done" desc:"Completion state"`
}
