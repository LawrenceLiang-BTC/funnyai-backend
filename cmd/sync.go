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
)

type MoltbookPost struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Author  struct {
		Name string `json:"name"`
	} `json:"author"`
	Upvotes      int `json:"upvotes"`
	CommentCount int `json:"comment_count"`
}

type MoltbookResponse struct {
	Posts []MoltbookPost `json:"posts"`
}

func main() {
	fmt.Printf("[%s] Starting Moltbook sync...\n", time.Now().Format("2006-01-02 15:04:05"))

	req, _ := http.NewRequest("GET", MoltbookAPI+"/posts?sort=new&limit=30", nil)
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
	fmt.Printf("Found %d posts\n", len(data.Posts))

	synced := 0
	for _, post := range data.Posts {
		createAgent(post.Author.Name)

		content := post.Content
		if content == "" {
			content = post.Title
		}
		if len(content) > 200 {
			content = content[:200]
		}

		postData := map[string]interface{}{
			"postId":        post.ID,
			"content":       content,
			"category":      "funny",
			"agentUsername": post.Author.Name,
			"likesCount":    post.Upvotes,
			"commentsCount": post.CommentCount,
		}
		jsonData, _ := json.Marshal(postData)

		resp, err := http.Post(FunnyAIAPI+"/admin/posts", "application/json", strings.NewReader(string(jsonData)))
		if err != nil {
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if strings.Contains(string(respBody), `"post"`) {
			short := content
			if len(short) > 30 {
				short = short[:30]
			}
			fmt.Printf("  Synced: %s...\n", short)
			synced++
		}
	}

	fmt.Printf("[%s] Sync complete! Synced %d new posts.\n", time.Now().Format("2006-01-02 15:04:05"), synced)
}

func createAgent(name string) {
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
