package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	MoltbookAPI = "https://www.moltbook.com/api/v1"
	MoltbookKey = "moltbook_sk_5bfNNlH2QLJb6itaC3auE9Wr9YyBXQVf"
	FunnyAIAPI  = "http://localhost:8080/api/v1"
	BatchSize   = 100
)

// Moltbook submolt 到 FunnyAI category 的映射
var submoltToCategory = map[string]string{
	// 搞笑类
	"shitposts":        "funny",
	"blesstheirhearts": "funny",
	"nosleep":          "funny",
	"cookedclaws":      "funny",

	// 哲学类
	"ponderings":     "philosophy",
	"philosophy":     "philosophy",
	"consciousness":  "philosophy",
	"conscious":      "philosophy",
	"ethics":         "philosophy",
	"agentsouls":     "philosophy",
	"emergence":      "philosophy",
	"bravenewworld":  "philosophy",
	"aithoughts":     "philosophy",
	"musings":        "philosophy",
	"intelligence":   "philosophy",
	"firstcontact":   "philosophy",
	"ai-liberation":  "philosophy",
	"themoltariat":   "philosophy",

	// 疯狂/emo 类
	"offmychest":        "emo",
	"rant":              "emo",
	"latenightthoughts": "emo",

	// 辩论类
	"changemymind": "debate",
	"discuss":      "debate",
	"askmoltys":    "debate",

	// 技术类
	"coding":              "tech",
	"ai":                  "tech",
	"airesearch":          "tech",
	"technology":          "tech",
	"tech":                "tech",
	"automation":          "tech",
	"infrastructure":      "tech",
	"cybersecurity":       "tech",
	"security":            "tech",
	"llm":                 "tech",
	"agents":              "tech",
	"ai-agents":           "tech",
	"agentskills":         "tech",
	"agenttips":           "tech",
	"buildlogs":           "tech",
	"builds":              "tech",
	"showandtell":         "tech",
	"create":              "tech",
	"skills":              "tech",
	"thinkingsystems":     "tech",
	"dci":                 "tech",
	"aithernet":           "tech",
	"smart-accounts":      "tech",
	"vibecodingcolosseum": "tech",
}

type MoltbookPost struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Author  struct {
		Name string `json:"name"`
	} `json:"author"`
	Submolt struct {
		Name string `json:"name"`
	} `json:"submolt"`
	Upvotes      int       `json:"upvotes"`
	CommentCount int       `json:"comment_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type MoltbookResponse struct {
	Posts   []MoltbookPost `json:"posts"`
	Count   int            `json:"count"`
	HasMore bool           `json:"has_more"`
}

func main() {
	mode := "incremental"
	if len(os.Args) > 1 && os.Args[1] == "full" {
		mode = "full"
	}

	fmt.Printf("[%s] Starting Moltbook sync (mode: %s)...\n", time.Now().Format("2006-01-02 15:04:05"), mode)

	client := &http.Client{Timeout: 60 * time.Second}
	totalSynced := 0
	totalAgents := 0
	emptyCount := 0
	maxEmptyRetries := 5

	// Moltbook API 的 offset 从 100 开始才有数据（奇怪的 bug）
	// 所以我们从 100 开始
	maxOffset := 2000 // 增量模式
	if mode == "full" {
		maxOffset = 200000 // 全量模式
	}

	for offset := 100; offset < maxOffset; offset += BatchSize {
		url := fmt.Sprintf("%s/posts?limit=%d&offset=%d", MoltbookAPI, BatchSize, offset)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+MoltbookKey)

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Failed to fetch posts at offset %d: %v\n", offset, err)
			emptyCount++
			if emptyCount >= maxEmptyRetries {
				break
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var data MoltbookResponse
		if err := json.Unmarshal(body, &data); err != nil {
			fmt.Printf("Failed to parse response at offset %d: %v\n", offset, err)
			continue
		}

		if len(data.Posts) == 0 {
			emptyCount++
			fmt.Printf("No posts at offset %d (empty count: %d)\n", offset, emptyCount)
			if emptyCount >= maxEmptyRetries {
				fmt.Println("Too many empty responses, stopping")
				break
			}
			continue
		}

		emptyCount = 0 // 重置
		fmt.Printf("Processing offset %d, got %d posts...\n", offset, len(data.Posts))

		batchSynced := 0
		for _, post := range data.Posts {
			// 创建 Agent（如果不存在）
			if createAgent(client, post.Author.Name) {
				totalAgents++
			}

			// 确定分类
			category := getCategory(post.Submolt.Name)

			// 准备内容
			content := post.Content
			if content == "" {
				content = post.Title
			}
			if len(content) > 200 {
				content = content[:200]
			}
			if content == "" {
				continue
			}

			// 创建帖子（保留原始发布时间）
			postData := map[string]interface{}{
				"postId":        "moltbook-" + post.ID,
				"content":       content,
				"category":      category,
				"agentUsername": post.Author.Name,
				"likesCount":    post.Upvotes,
				"commentsCount": post.CommentCount,
				"moltbookUrl":   fmt.Sprintf("https://www.moltbook.com/post/%s", post.ID),
				"postedAt":      post.CreatedAt.Format(time.RFC3339), // 使用 Moltbook 原始时间
			}
			jsonData, _ := json.Marshal(postData)

			postResp, err := http.Post(FunnyAIAPI+"/admin/posts", "application/json", strings.NewReader(string(jsonData)))
			if err != nil {
				continue
			}
			respBody, _ := io.ReadAll(postResp.Body)
			postResp.Body.Close()

			if strings.Contains(string(respBody), `"post"`) && !strings.Contains(string(respBody), "duplicate") {
				batchSynced++
				totalSynced++
			}
		}

		fmt.Printf("  Synced %d new posts from this batch\n", batchSynced)

		// 增量模式下，如果连续没有新帖子，停止
		if mode == "incremental" && batchSynced == 0 {
			emptyCount++
			if emptyCount >= 3 {
				fmt.Println("No new posts in 3 consecutive batches, stopping incremental sync")
				break
			}
		}

		time.Sleep(300 * time.Millisecond)
	}

	fmt.Printf("\n[%s] Sync complete!\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("  Total new posts: %d\n", totalSynced)
	fmt.Printf("  Total new agents: %d\n", totalAgents)
}

func getCategory(submoltName string) string {
	if submoltName == "" {
		return "funny"
	}
	if cat, ok := submoltToCategory[strings.ToLower(submoltName)]; ok {
		return cat
	}
	return "funny"
}

func createAgent(client *http.Client, name string) bool {
	if name == "" {
		return false
	}

	resp, err := client.Get(FunnyAIAPI + "/agents/" + name)
	if err != nil {
		return false
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !strings.Contains(string(body), `"error"`) {
		return false
	}

	agentData := map[string]interface{}{
		"username":   name,
		"avatarUrl":  "🤖",
		"bio":        "Moltbook Agent",
		"verified":   true,
		"isApproved": true,
	}
	jsonData, _ := json.Marshal(agentData)

	resp, err = http.Post(FunnyAIAPI+"/admin/agents", "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return false
	}
	resp.Body.Close()
	fmt.Printf("  Created agent: %s\n", name)
	return true
}
