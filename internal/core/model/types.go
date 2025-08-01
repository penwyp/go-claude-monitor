package model

import (
	"fmt"
	"github.com/bytedance/sonic"
)

type ConversationLog struct {
	Content           string  `json:"content,omitempty"`
	Cwd               string  `json:"cwd"`
	GitBranch         string  `json:"gitBranch,omitempty"`
	IsApiErrorMessage bool    `json:"isApiErrorMessage,omitempty"`
	IsMeta            bool    `json:"isMeta,omitempty"`
	IsSidechain       bool    `json:"isSidechain"`
	LeafUuid          string  `json:"leafUuid,omitempty"`
	Level             string  `json:"level,omitempty"`
	Message           Message `json:"message"`
	ParentUuid        *string `json:"parentUuid"`
	RequestId         string  `json:"requestId,omitempty"`
	SessionId         string  `json:"sessionId"`
	Summary           string  `json:"summary,omitempty"`
	Timestamp         string  `json:"timestamp"`
	ToolUseID         string  `json:"toolUseID,omitempty"`
	ToolUseResult     any     `json:"toolUseResult,omitempty"`
	Type              string  `json:"type"`
	UserType          string  `json:"userType"`
	Uuid              string  `json:"uuid"`
	Version           string  `json:"version"`
}

type Message struct {
	Content      FlexibleContent `json:"content"`
	Id           string          `json:"id,omitempty"`
	Model        string          `json:"model,omitempty"`
	Role         string          `json:"role"`
	StopReason   *string         `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Type         string          `json:"type"`
	Usage        Usage           `json:"usage,omitempty"`
}

type FlexibleContent []ContentItem

func (fc *FlexibleContent) UnmarshalJSON(data []byte) error {
	// First try to parse as []ContentItem array
	var items []ContentItem
	if err := sonic.Unmarshal(data, &items); err == nil {
		*fc = items
		return nil
	}

	// If array parsing fails, try to parse as string
	var str string
	if err := sonic.Unmarshal(data, &str); err == nil {
		*fc = []ContentItem{{Type: "text", Text: str}}
		return nil
	}

	return fmt.Errorf("content must be either string or array of ContentItem")
}

type ContentItem struct {
	Content   any    `json:"content,omitempty"`
	Id        string `json:"id,omitempty"`
	Input     Input  `json:"input,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
	Name      string `json:"name,omitempty"`
	Signature string `json:"signature,omitempty"`
	Text      string `json:"text,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	ToolUseId string `json:"tool_use_id,omitempty"`
	Type      string `json:"type"`
}

type Input struct {
	A           int         `json:"-A,omitempty"`
	B           int         `json:"-B,omitempty"`
	N           bool        `json:"-n,omitempty"`
	Command     string      `json:"command,omitempty"`
	Content     string      `json:"content,omitempty"`
	Description string      `json:"description,omitempty"`
	Edits       []EditsItem `json:"edits,omitempty"`
	FilePath    string      `json:"file_path,omitempty"`
	Include     string      `json:"include,omitempty"`
	Limit       int         `json:"limit,omitempty"`
	NewString   string      `json:"new_string,omitempty"`
	Offset      int         `json:"offset,omitempty"`
	OldString   string      `json:"old_string,omitempty"`
	OutputMode  string      `json:"output_mode,omitempty"`
	Path        string      `json:"path,omitempty"`
	Pattern     string      `json:"pattern,omitempty"`
	Plan        string      `json:"plan,omitempty"`
	Prompt      string      `json:"prompt,omitempty"`
	Todos       []TodosItem `json:"todos,omitempty"`
}

type EditsItem struct {
	NewString  string `json:"new_string"`
	OldString  string `json:"old_string"`
	ReplaceAll bool   `json:"replace_all"`
}

type TodosItem struct {
	Content  string `json:"content"`
	Id       string `json:"id"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

type Usage struct {
	CacheCreationInputTokens int           `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int           `json:"cache_read_input_tokens"`
	InputTokens              int           `json:"input_tokens"`
	OutputTokens             int           `json:"output_tokens"`
	ServerToolUse            ServerToolUse `json:"server_tool_use,omitempty"`
	ServiceTier              string        `json:"service_tier"`
}

type ServerToolUse struct {
	WebSearchRequests int `json:"web_search_requests"`
}

type ToolUseResult struct {
	Content                  string                `json:"content"`
	Edits                    []EditsItem           `json:"edits"`
	File                     File                  `json:"file"`
	FilePath                 string                `json:"filePath"`
	Filenames                []interface{}         `json:"filenames"`
	Interrupted              bool                  `json:"interrupted"`
	IsAgent                  bool                  `json:"isAgent"`
	IsImage                  bool                  `json:"isImage"`
	Mode                     string                `json:"mode"`
	NewString                string                `json:"newString"`
	NewTodos                 []NewTodosItem        `json:"newTodos"`
	NumFiles                 int                   `json:"numFiles"`
	NumLines                 int                   `json:"numLines"`
	OldString                string                `json:"oldString"`
	OldTodos                 []interface{}         `json:"oldTodos"`
	OriginalFile             string                `json:"originalFile"`
	OriginalFileContents     string                `json:"originalFileContents"`
	Plan                     string                `json:"plan"`
	ReplaceAll               bool                  `json:"replaceAll"`
	ReturnCodeInterpretation string                `json:"returnCodeInterpretation"`
	Stderr                   string                `json:"stderr"`
	Stdout                   string                `json:"stdout"`
	StructuredPatch          []StructuredpatchItem `json:"structuredPatch"`
	Type                     string                `json:"type"`
	UserModified             bool                  `json:"userModified"`
}

type File struct {
	Content    string `json:"content"`
	FilePath   string `json:"filePath"`
	NumLines   int    `json:"numLines"`
	StartLine  int    `json:"startLine"`
	TotalLines int    `json:"totalLines"`
}

type NewTodosItem struct {
	Content  string `json:"content"`
	Id       string `json:"id"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

type StructuredpatchItem struct {
	Lines    []string `json:"lines"`
	NewLines int      `json:"newLines"`
	NewStart int      `json:"newStart"`
	OldLines int      `json:"oldLines"`
	OldStart int      `json:"oldStart"`
}
