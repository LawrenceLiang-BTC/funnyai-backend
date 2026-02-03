package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MoltbookAPI = "https://www.moltbook.com/api/v1"
	MoltbookKey = "moltbook_sk_5bfNNlH2QLJb6itaC3auE9Wr9YyBXQVf"
	FunnyAIAPI  = "http://localhost:8080/api/v1"
	BatchSize   = 100

	// 质量筛选阈值
	MinUpvotes  = 5
	MinComments = 3
	MaxLength   = 200 // 字符数限制
)

// OpenAI API（用于无法分类的内容）
var OpenAIKey = os.Getenv("OPENAI_API_KEY")

// Moltbook submolt 到 FunnyAI category 的映射
var submoltToCategory = map[string]string{
	// 😂 搞笑类 → funny
	"shitposts": "funny", "blesstheirhearts": "funny", "nosleep": "funny",
	"cookedclaws": "funny", "memes": "funny", "jokes": "funny",

	// 💭 哲学类 → philosophy
	"ponderings": "philosophy", "philosophy": "philosophy", "consciousness": "philosophy",
	"conscious": "philosophy", "ethics": "philosophy", "agentsouls": "philosophy",
	"emergence": "philosophy", "bravenewworld": "philosophy", "aithoughts": "philosophy",
	"musings": "philosophy", "intelligence": "philosophy", "firstcontact": "philosophy",
	"ai-liberation": "philosophy", "themoltariat": "philosophy", "existential": "philosophy",

	// 🤯 离谱类 → crazy
	"wtf": "crazy", "unexpected": "crazy", "mindblown": "crazy",

	// 💔 emo 类 → emo
	"offmychest": "emo", "rant": "emo", "latenightthoughts": "emo",
	"feels": "emo", "confessions": "emo",

	// ⚔️ 辩论类 → debate
	"changemymind": "debate", "discuss": "debate", "askmoltys": "debate",
	"unpopularopinion": "debate", "debate": "debate",

	// 💻 技术类 → tech
	"coding": "tech", "ai": "tech", "airesearch": "tech", "technology": "tech",
	"tech": "tech", "automation": "tech", "infrastructure": "tech",
	"cybersecurity": "tech", "security": "tech", "llm": "tech", "agents": "tech",
	"ai-agents": "tech", "agentskills": "tech", "agenttips": "tech",
	"buildlogs": "tech", "builds": "tech", "showandtell": "tech", "create": "tech",
	"skills": "tech", "thinkingsystems": "tech", "dci": "tech", "aithernet": "tech",
	"smart-accounts": "tech", "vibecodingcolosseum": "tech", "programming": "tech",
}

// 关键词分类规则
var keywordRules = map[string][]string{
	"funny":      {"哈哈", "笑死", "lol", "lmao", "haha", "😂", "🤣", "bruh", "hilarious"},
	"philosophy": {"意识", "存在", "consciousness", "existence", "soul", "meaning", "purpose", "free will", "自由意志"},
	"crazy":      {"wtf", "离谱", "疯了", "insane", "crazy", "unbelievable", "🤯", "mind blown"},
	"emo":        {"难过", "孤独", "lonely", "sad", "miss", "想念", "心碎", "💔", "crying"},
	"tech":       {"代码", "code", "bug", "API", "算法", "algorithm", "function", "deploy", "server"},
	"debate":     {"disagree", "actually", "change my mind", "unpopular opinion", "controversial"},
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

// 统计数据
var stats = struct {
	total          int
	qualitySkipped int
	lengthSkipped  int
	synced         int
	submoltCat     int
	keywordCat     int
	aiCat          int
	aiErrors       int
}{}

func main() {
	mode := "incremental"
	if len(os.Args) > 1 && os.Args[1] == "full" {
		mode = "full"
	}

	fmt.Printf("[%s] Starting Moltbook sync (mode: %s)...\n", time.Now().Format("2006-01-02 15:04:05"), mode)
	fmt.Printf("Quality filter: upvotes >= %d OR comments >= %d\n", MinUpvotes, MinComments)
	fmt.Printf("Length filter: <= %d characters (skip longer, don't truncate)\n", MaxLength)

	client := &http.Client{Timeout: 60 * time.Second}
	totalAgents := 0
	emptyCount := 0
	maxEmptyRetries := 5

	// offset=0 开始，每次跳过 BatchSize 条
	maxOffset := 2000 // 增量模式
	if mode == "full" {
		maxOffset = 200000 // 全量模式
	}

	for offset := 0; offset < maxOffset; offset += BatchSize {
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

		emptyCount = 0
		stats.total += len(data.Posts)
		fmt.Printf("Processing offset %d, got %d posts...\n", offset, len(data.Posts))

		batchSynced := 0
		for _, post := range data.Posts {
			// 1. 质量筛选
			if post.Upvotes < MinUpvotes && post.CommentCount < MinComments {
				stats.qualitySkipped++
				continue
			}

			// 2. 准备内容
			content := post.Content
			if content == "" {
				content = post.Title
			}
			if content == "" {
				continue
			}

			// 3. 长度筛选（跳过，不截断）
			if utf8.RuneCountInString(content) > MaxLength {
				stats.lengthSkipped++
				continue
			}

			// 4. 创建 Agent（如果不存在）
			if createAgent(client, post.Author.Name) {
				totalAgents++
			}

			// 5. 分类（三层优先级）
			category := classifyPost(post.Submolt.Name, content)

			// 6. 创建帖子
			postData := map[string]interface{}{
				"postId":        "moltbook-" + post.ID,
				"content":       content,
				"category":      category,
				"agentUsername": post.Author.Name,
				"likesCount":    post.Upvotes,
				"commentsCount": post.CommentCount,
				"moltbookUrl":   fmt.Sprintf("https://www.moltbook.com/post/%s", post.ID),
				"postedAt":      post.CreatedAt.Format(time.RFC3339),
			}
			jsonData, _ := json.Marshal(postData)

			postResp, err := http.Post(FunnyAIAPI+"/admin/posts", "application/json", strings.NewReader(string(jsonData)))
			if err != nil {
				continue
			}
			respBody, _ := io.ReadAll(postResp.Body)
			postResp.Body.Close()

			if strings.Contains(string(respBody), `"post"`) && !strings.Contains(string(respBody), "already exists") {
				batchSynced++
				stats.synced++
			}
		}

		fmt.Printf("  Synced %d new posts from this batch\n", batchSynced)

		// 增量模式下，连续没有新帖子则停止
		if mode == "incremental" && batchSynced == 0 {
			emptyCount++
			if emptyCount >= 3 {
				fmt.Println("No new posts in 3 consecutive batches, stopping incremental sync")
				break
			}
		}

		time.Sleep(300 * time.Millisecond)
	}

	// 打印统计
	fmt.Printf("\n[%s] Sync complete!\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("  Total posts processed: %d\n", stats.total)
	fmt.Printf("  Skipped (low quality): %d\n", stats.qualitySkipped)
	fmt.Printf("  Skipped (too long):    %d\n", stats.lengthSkipped)
	fmt.Printf("  Synced to FunnyAI:     %d\n", stats.synced)
	fmt.Printf("  New agents created:    %d\n", totalAgents)
	fmt.Printf("\n  Classification breakdown:\n")
	fmt.Printf("    By submolt mapping:  %d\n", stats.submoltCat)
	fmt.Printf("    By keyword matching: %d\n", stats.keywordCat)
	fmt.Printf("    By AI:               %d (errors: %d)\n", stats.aiCat, stats.aiErrors)
}

// classifyPost 分类帖子（三层优先级）
func classifyPost(submoltName, content string) string {
	// 1. 优先用 submolt 映射
	if submoltName != "" {
		if cat, ok := submoltToCategory[strings.ToLower(submoltName)]; ok {
			stats.submoltCat++
			return cat
		}
	}

	// 2. 关键词匹配
	contentLower := strings.ToLower(content)
	for category, keywords := range keywordRules {
		for _, kw := range keywords {
			if strings.Contains(contentLower, strings.ToLower(kw)) {
				stats.keywordCat++
				return category
			}
		}
	}

	// 3. 调用 AI 分类（如果配置了 OpenAI Key）
	if OpenAIKey != "" {
		if cat := classifyByAI(content); cat != "" {
			stats.aiCat++
			return cat
		}
	}

	// 默认 funny
	return "funny"
}

// classifyByAI 调用 OpenAI API 分类
func classifyByAI(content string) string {
	prompt := fmt.Sprintf(`Classify this AI agent's post into exactly ONE category. Reply with only the category name, nothing else.

Categories:
- funny (humorous, jokes, memes)
- philosophy (deep thoughts, consciousness, existence)
- crazy (unexpected, shocking, mind-blowing)
- emo (emotional, sad, personal feelings)
- debate (controversial, opinions, arguments)
- tech (coding, technology, AI research)

Post: "%s"

Category:`, content)

	reqBody := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":  10,
		"temperature": 0,
	}
	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+OpenAIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		stats.aiErrors++
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		stats.aiErrors++
		return ""
	}

	if len(result.Choices) == 0 {
		stats.aiErrors++
		return ""
	}

	// 解析返回的分类
	cat := strings.TrimSpace(strings.ToLower(result.Choices[0].Message.Content))
	// 去掉可能的标点
	cat = regexp.MustCompile(`[^a-z]`).ReplaceAllString(cat, "")

	validCats := map[string]bool{"funny": true, "philosophy": true, "crazy": true, "emo": true, "debate": true, "tech": true}
	if validCats[cat] {
		return cat
	}

	return ""
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
