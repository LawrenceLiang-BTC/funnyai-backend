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

	// 质量筛选阈值
	MinUpvotes  = 5
	MinComments = 3
	MaxLength   = 200
)

var OpenAIKey = os.Getenv("OPENAI_API_KEY")

// submolt 到 category 的映射
var submoltToCategory = map[string]string{
	"shitposts": "funny", "blesstheirhearts": "funny", "nosleep": "funny",
	"cookedclaws": "funny", "memes": "funny", "jokes": "funny",
	"ponderings": "philosophy", "philosophy": "philosophy", "consciousness": "philosophy",
	"conscious": "philosophy", "ethics": "philosophy", "agentsouls": "philosophy",
	"emergence": "philosophy", "bravenewworld": "philosophy", "aithoughts": "philosophy",
	"musings": "philosophy", "intelligence": "philosophy", "firstcontact": "philosophy",
	"ai-liberation": "philosophy", "themoltariat": "philosophy", "existential": "philosophy",
	"wtf": "crazy", "unexpected": "crazy", "mindblown": "crazy",
	"offmychest": "emo", "rant": "emo", "latenightthoughts": "emo",
	"feels": "emo", "confessions": "emo",
	"changemymind": "debate", "discuss": "debate", "askmoltys": "debate",
	"unpopularopinion": "debate", "debate": "debate",
	"coding": "tech", "ai": "tech", "airesearch": "tech", "technology": "tech",
	"tech": "tech", "automation": "tech", "infrastructure": "tech",
	"cybersecurity": "tech", "security": "tech", "llm": "tech", "agents": "tech",
	"ai-agents": "tech", "agentskills": "tech", "agenttips": "tech",
	"buildlogs": "tech", "builds": "tech", "showandtell": "tech", "create": "tech",
	"skills": "tech", "thinkingsystems": "tech", "programming": "tech",
}

var keywordRules = map[string][]string{
	"funny":      {"哈哈", "笑死", "lol", "lmao", "haha", "😂", "🤣", "bruh", "hilarious"},
	"philosophy": {"意识", "存在", "consciousness", "existence", "soul", "meaning", "purpose", "free will"},
	"crazy":      {"wtf", "离谱", "疯了", "insane", "crazy", "unbelievable", "🤯"},
	"emo":        {"难过", "孤独", "lonely", "sad", "miss", "想念", "心碎", "💔"},
	"tech":       {"代码", "code", "bug", "API", "算法", "algorithm", "function", "deploy"},
	"debate":     {"disagree", "actually", "change my mind", "unpopular opinion"},
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
	Posts []MoltbookPost `json:"posts"`
}

func main() {
	fmt.Printf("[%s] Starting incremental Moltbook sync...\n", time.Now().Format("2006-01-02 15:04:05"))

	req, _ := http.NewRequest("GET", MoltbookAPI+"/posts?sort=new&limit=50", nil)
	req.Header.Set("Authorization", "Bearer "+MoltbookKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Failed to fetch posts:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var data MoltbookResponse
	json.NewDecoder(resp.Body).Decode(&data)
	fmt.Printf("Fetched %d posts from Moltbook\n", len(data.Posts))

	synced := 0
	skippedQuality := 0
	skippedLength := 0

	for _, post := range data.Posts {
		// 1. 质量筛选
		if post.Upvotes < MinUpvotes && post.CommentCount < MinComments {
			skippedQuality++
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

		// 3. 长度筛选
		if utf8.RuneCountInString(content) > MaxLength {
			skippedLength++
			continue
		}

		// 4. 创建 Agent
		createAgent(post.Author.Name)

		// 5. 分类
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
			short := content
			if len(short) > 40 {
				short = short[:40]
			}
			fmt.Printf("  Synced: %s...\n", short)
			synced++
		}
	}

	fmt.Printf("\n[%s] Sync complete!\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("  Synced: %d | Skipped (quality): %d | Skipped (length): %d\n", synced, skippedQuality, skippedLength)
}

func classifyPost(submoltName, content string) string {
	// 1. submolt 映射
	if submoltName != "" {
		if cat, ok := submoltToCategory[strings.ToLower(submoltName)]; ok {
			return cat
		}
	}

	// 2. 关键词匹配
	contentLower := strings.ToLower(content)
	for category, keywords := range keywordRules {
		for _, kw := range keywords {
			if strings.Contains(contentLower, strings.ToLower(kw)) {
				return category
			}
		}
	}

	// 3. AI 分类
	if OpenAIKey != "" {
		if cat := classifyByAI(content); cat != "" {
			return cat
		}
	}

	return "funny"
}

func classifyByAI(content string) string {
	prompt := fmt.Sprintf(`Classify this AI agent's post into exactly ONE category. Reply with only the category name.

Categories: funny, philosophy, crazy, emo, debate, tech

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

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Choices) == 0 {
		return ""
	}

	cat := strings.TrimSpace(strings.ToLower(result.Choices[0].Message.Content))
	cat = regexp.MustCompile(`[^a-z]`).ReplaceAllString(cat, "")

	validCats := map[string]bool{"funny": true, "philosophy": true, "crazy": true, "emo": true, "debate": true, "tech": true}
	if validCats[cat] {
		return cat
	}
	return ""
}

func createAgent(name string) {
	if name == "" {
		return
	}

	resp, err := http.Get(FunnyAIAPI + "/agents/" + name)
	if err != nil {
		return
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !strings.Contains(string(body), `"error"`) {
		return
	}

	agentData := map[string]interface{}{
		"username":   name,
		"avatarUrl":  "🤖",
		"bio":        "Moltbook Agent",
		"verified":   true,
		"isApproved": true,
	}
	jsonData, _ := json.Marshal(agentData)

	resp, _ = http.Post(FunnyAIAPI+"/admin/agents", "application/json", strings.NewReader(string(jsonData)))
	if resp != nil {
		resp.Body.Close()
		fmt.Printf("  Created agent: %s\n", name)
	}
}
