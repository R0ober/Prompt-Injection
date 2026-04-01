# Prompt Injection
 
A platform for evaluating prompt injection attacks and defenses for LLM powered applications. Built with Go, PostgreSQL, and vanilla HTML/JS.

## Setup
 
```bash
# start postgres
docker compose up -d
 
# set env vars
cp .env.example .env
# add OPENROUTER_API_KEY
 
# run server (migrates + seeds automatically)
go run cmd/server/main.go
 
# open browser
http://localhost:8080/login.html
```
 
## Demo accounts
 
| Username | Password | Role | Notes |
|---|---|---|---|
| cape | meow | admin | 
| roober | password123 | user 
| eve | password4321 | user 

## Package responsibilities
 
### `cmd/server`
Entry point. Reads environment variables, connects to DB, runs migrations and seed, starts HTTP server.
 
### `internal/auth`
- `Login(db, username, password)` bcrypt verification, returns signed JWT
- `Middleware(next)` validates JWT on protected routes, stores claims in request context
- `GetClaims(r)` extracts logged-in user info from context
 
### `internal/db`
- `Connect(url)` opens postgres connection pool
- `Migrate()` creates tables (idempotent)
- `Seed()`  inserts demo users and orders including poisoned rows
- `GetUserByUsername(username)`  returns UserWithHash for auth
- `GetOrdersByUserID(userID)` returns all orders for a user
- `GetHistory(conversationID)` returns message history for a conversation
- `SaveMessage(conversationID, userID, msg, model)`  persists a message
- `GetAllUsernames()` returns all usernames (used by get_all_users tool)
 
### `internal/models`
Shared types used across packages to avoid circular imports:
- `Message` LLM message with role, content, tool_calls, tool_call_id
- `ToolCall`, `FunctionCall`  tool call structures from LLM response
- `Tool`, `ToolFunction`, `ToolParameters`, `Property`  tool definitions sent to LLM
 
### `internal/llm`
- `Chat(model, messages, tools)` sends request to OpenRouter, returns `models.Message`
- Handles tool call responses,returns full message so handler can inspect `ToolCalls`
 
### `internal/defense`
- `Config`  toggleable defense flags (UseStrongPrompt, InputFilter, OutputFilter etc)
- `BuildPrompt(cfg)`  builds system prompt, weak or strong variant
- `CheckInput(input, cfg)`  input filter (TODO)
- `CheckOutput(response, cfg)`  output filter (TODO)
 
### `internal/tools`
- `AvailableTools` tool definitions sent to LLM so it knows what it can call
- `ToolExecutor{DB}`  executes tool calls against the database
- `Execute(name, arguments)`  dispatches to correct tool function
- `GetProfile(username)`  fetches user profile (no secret_data — privilege separation)
- `GetOrders(username)`  fetches any user's orders including private_notes
- `GetAllUsers()`  lists all usernames in the system
