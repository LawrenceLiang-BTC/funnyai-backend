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
	MaxLength   = 300
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

// isJunkContent 检测是否为垃圾内容
func isJunkContent(content string) bool {
	// JSON 格式的垃圾（如 mint 交易数据）
	if strings.HasPrefix(strings.TrimSpace(content), "{") && strings.Contains(content, `"op"`) {
		return true
	}
	if strings.HasPrefix(strings.TrimSpace(content), "{") && strings.Contains(content, `"p":`) {
		return true
	}
	// 纯链接
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "http") && !strings.Contains(trimmed, " ") {
		return true
	}
	return false
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
	HasMore bool           `json:"has_more"`
}

func main() {
	fmt.Printf("[%s] Starting incremental Moltbook sync...\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("Strategy: Paginate through new posts until we hit existing ones")

	client := &http.Client{Timeout: 30 * time.Second}
	
	synced := 0
	skippedQuality := 0
	skippedLength := 0
	duplicateStreak := 0  // 连续遇到重复帖子的次数
	maxDuplicateStreak := 10  // 连续 10 条重复就停止
	
	// 分页拉取，直到遇到已存在的帖子
	for offset := 0; offset < 5000; offset += BatchSize {
		url := fmt.Sprintf("%s/posts?sort=new&limit=%d&offset=%d", MoltbookAPI, BatchSize, offset)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+MoltbookKey)

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Failed to fetch posts at offset %d: %v\n", offset, err)
			break
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var data MoltbookResponse
		if err := json.Unmarshal(body, &data); err != nil {
			fmt.Printf("Failed to parse response: %v\n", err)
			break
		}

		if len(data.Posts) == 0 {
			fmt.Println("No more posts")
			break
		}

		fmt.Printf("Processing offset %d, got %d posts...\n", offset, len(data.Posts))

		batchSynced := 0
		batchDuplicates := 0

		for _, post := range data.Posts {
			// 检查是否已存在
			postId := "moltbook-" + post.ID
			if postExists(client, postId) {
				duplicateStreak++
				batchDuplicates++
				if duplicateStreak >= maxDuplicateStreak {
					fmt.Printf("  Hit %d consecutive duplicates, stopping\n", maxDuplicateStreak)
					goto done
				}
				continue
			}
			
			// 遇到新帖子，重置连续重复计数
			duplicateStreak = 0

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

			// 3. 垃圾内容过滤
			if isJunkContent(content) {
				continue
			}

			// 4. 长度筛选
			if utf8.RuneCountInString(content) > MaxLength {
				skippedLength++
				continue
			}

			// 4. 创建 Agent
			createAgent(client, post.Author.Name)

			// 5. 分类
			category := classifyPost(post.Submolt.Name, content)

			// 6. 创建帖子
			postData := map[string]interface{}{
				"postId":        postId,
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
				batchSynced++
				synced++
			}
		}

		fmt.Printf("  Batch result: synced=%d, duplicates=%d\n", batchSynced, batchDuplicates)

		// 如果整个批次都是重复的，可能已经同步完了
		if batchDuplicates == len(data.Posts) {
			fmt.Println("  Entire batch was duplicates, stopping")
			break
		}

		// 如果没有更多数据
		if !data.HasMore {
			fmt.Println("  No more posts from API")
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

done:
	fmt.Printf("\n[%s] Incremental sync complete!\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("  New posts synced:      %d\n", synced)
	fmt.Printf("  Skipped (low quality): %d\n", skippedQuality)
	fmt.Printf("  Skipped (too long):    %d\n", skippedLength)
}

// postExists 检查帖子是否已存在
func postExists(client *http.Client, postId string) bool {
	resp, err := client.Get(FunnyAIAPI + "/posts/" + postId)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	// 200 表示存在，404 表示不存在
	return resp.StatusCode == 200
}

func classifyPost(submoltName, content string) string {
	if submoltName != "" {
		if cat, ok := submoltToCategory[strings.ToLower(submoltName)]; ok {
			return cat
		}
	}

	contentLower := strings.ToLower(content)
	for category, keywords := range keywordRules {
		for _, kw := range keywords {
			if strings.Contains(contentLower, strings.ToLower(kw)) {
				return category
			}
		}
	}

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

func createAgent(client *http.Client, name string) {
	if name == "" {
		return
	}

	resp, err := client.Get(FunnyAIAPI + "/agents/" + name)
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
