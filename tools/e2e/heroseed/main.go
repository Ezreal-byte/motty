// Command heroseed writes the assistant conversation the landing page's hero
// shot shows into a fresh assistant database, so the shot needs no model, no
// API key, and no network: the exchange is authored here and the app renders it
// through the same restore path as any saved conversation. The tool card quotes
// the tail of the fixture's log, so the card and the terminal agree.
//
//	go run ./tools/e2e/heroseed <path to ai.db> <path to logs/app.log>
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"motty/internal/ai/llm"
	"motty/internal/ai/store"
	"motty/internal/ai/tools"
)

// The provider and model the shot's chip names. No key: the conversation in the
// panel was restored, not fetched.
const (
	providerID = "hero-deepseek"
	modelName  = "deepseek-flash"
	session    = "staging-web"
	// toolLines is how much of the log the card's read covers.
	toolLines = 38
)

const question = `帮我看下 staging-web 最近的日志,有没有要马上处理的问题?`

const answer = `看完了。38 行里 36 条 INFO、2 条 WARN,没有 5xx,这一屏可以放心:

- 18:08:42 WARN slow query table=sessions ms=412:sessions 表这条查询用了 412ms。只出现一次、量级不大,先记成观察项,之后再频繁出现就该加索引。
- 18:09:24 WARN retry attempt=2 upstream=auth waited=1500ms:auth 上游超时后重试了一次,第二次就过了,属于偶发抖动,不用现在处理。

另外两点:

- status 都在 2xx(两次 201 创建、一条 101 协议升级),一条 5xx 都没有。
- 请求间隔 4 到 12 秒,没有突然的空档或堆积,发布之后是平稳接流的。

要我在整个 logs/ 目录里统计一下 ERROR/WARN 的条数,或者拿 releases/ 里最近两次发布做个对比吗?`

// The tool call the assistant made: read the session's scrollback. Its card is
// what the shot shows above the answer.
const (
	toolCallID = "call_hero_1"
	toolArgs   = `{"scope":"scrollback","lines":200}`
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: heroseed <path to ai.db> <path to logs/app.log>")
	}
	path, logPath := os.Args[1], os.Args[2]
	db, err := store.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.SaveProvider(store.Provider{
		ID: providerID, Name: "DeepSeek", Kind: store.KindOpenAI,
		BaseURL: "https://api.deepseek.com/v1", Enabled: true,
	}); err != nil {
		log.Fatal(err)
	}
	if err := db.SaveModels(providerID, []store.Model{{ProviderID: providerID, Name: modelName, Enabled: true}}); err != nil {
		log.Fatal(err)
	}
	if err := db.SetDefaultModel(providerID, modelName); err != nil {
		log.Fatal(err)
	}

	result, err := tailOf(logPath, toolLines)
	if err != nil {
		log.Fatal(err)
	}
	call, err := json.Marshal(struct {
		Calls []llm.ToolCall `json:"calls,omitempty"`
	}{Calls: []llm.ToolCall{{ID: toolCallID, Name: tools.NameTerminalRead, Arguments: toolArgs}}})
	if err != nil {
		log.Fatal(err)
	}
	callID, err := json.Marshal(struct {
		CallID string `json:"call_id,omitempty"`
	}{CallID: toolCallID})
	if err != nil {
		log.Fatal(err)
	}

	conversation, err := db.CreateConversation(question, session)
	if err != nil {
		log.Fatal(err)
	}
	for _, message := range []store.StoredMessage{
		{Role: string(llm.RoleUser), Text: question},
		{Role: string(llm.RoleAssistant), ToolJSON: call},
		{Role: string(llm.RoleTool), Text: result, ToolJSON: callID},
		{Role: string(llm.RoleAssistant), Text: answer},
	} {
		if _, err := db.AppendMessage(conversation.ID, message); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("seeded conversation %d in %s\n", conversation.ID, path)
}

// tailOf is the last lines of a log under the command that printed them, which
// is what a terminal read puts in the card.
func tailOf(path string, lines int) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	kept := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(kept) > lines {
		kept = kept[len(kept)-lines:]
	}
	return "deploy@staging-web:~/app$ ./scripts/logs.sh " + strconv.Itoa(lines) + "\n" +
		strings.Join(kept, "\n"), nil
}
