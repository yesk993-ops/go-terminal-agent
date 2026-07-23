# Plan: Make Agent Provide Structured, Context-Aware Answers

## Steps

- [x] Step 1: Create `internal/core/answer.go` - Query classification logic (AnswerFormat, ClassifyQuery)
- [x] Step 2: Create `internal/core/quality.go` - Response quality checking
- [x] Step 3: Update `cmd/agent/main.go` - Replace `systemPromptFor()` with smart classification using `ClassifyQuery()` + `SystemPromptForFormat()`
- [x] Step 4: Update `internal/tui/tui.go` - Apply smart prompt selection in TUI mode using `ClassifyQuery()`
- [x] Step 5: Build and test (`go build ./...`, `go test ./...`) - All tests pass

