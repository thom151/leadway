package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type StreamEvent struct {
	ID     string `json:"id"`
	Object string `json:"object"`
	Delta  struct {
		Content []struct {
			Index int    `json:"index"`
			Type  string `json:"type"`
			Text  struct {
				Value string `json:"value"`
			} `json:"text"`
		} `json:"content"`
	} `json:"delta"`
}

func genThread(c *openai.Client, agent_name string) (openai.Thread, error) {
	ctx := context.Background()

	threadRequest := openai.ThreadRequest{
		Messages: []openai.ThreadMessage{
			{
				Role:    "user",
				Content: "You are a clone of" + agent_name + " that works at a real south morang",
			},
		},
	}

	thread, err := c.CreateThread(ctx, threadRequest)
	if err != nil {
		return openai.Thread{}, err
	}

	return thread, nil
}

func streamOpenAIResponse(cfg *apiConfig, threadID, clientMessage string, chunkChan chan<- string) (string, error) {
	ctx := context.Background()
	_, err := cfg.openaiClient.CreateMessage(ctx, threadID, openai.MessageRequest{
		Role:    "user",
		Content: clientMessage,
	})

	if err != nil {
		return "", err
	}

	url := "https://api.openai.com/v1/threads/" + threadID + "/runs"
	reqBody := map[string]interface{}{
		"assistant_id": cfg.assistantID,
		"stream":       true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("Create message failed: %v", err)
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.openaiApiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status: %d - %s", resp.StatusCode, string(body))
	}
	log.Println(http.StatusOK)

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var event StreamEvent
			err := json.Unmarshal([]byte(data), &event)
			if err != nil {
				continue
			}
			if event.Object == "thread.message.delta" && len(event.Delta.Content) > 0 {
				chunk := event.Delta.Content[0].Text.Value
				fullResponse.WriteString(chunk)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}
	close(chunkChan)
	return fullResponse.String(), scanner.Err()

}

func sendMessage(c *openai.Client, thread_id, client_message string) error {
	fmt.Println("sending message using: ", thread_id)
	request := openai.MessageRequest{
		Role:    "user",
		Content: client_message,
	}

	_, err := c.CreateMessage(context.Background(), thread_id, request)
	if err != nil {
		return err
	}

	return nil
}

func getRunID(c *openai.Client, thread_id, assistant_id string) (string, error) {
	run, err := c.CreateRun(context.Background(), thread_id, openai.RunRequest{
		AssistantID: assistant_id,
	})

	if err != nil {
		return "", err
	}

	return run.ID, nil
}

func getResponse(c *openai.Client, thread_id, run_id string) (string, error) {
	for {
		run, err := c.RetrieveRun(context.Background(), thread_id, run_id)
		if err != nil {
			return "", err
		}

		if run.Status == "completed" {
			messagesList, err := c.ListMessage(context.Background(), thread_id, nil, nil, nil, nil, nil)
			if err != nil {
				return "", err
			}

			message := messagesList.Messages[0].Content[0].Text.Value
			return message, nil
		}
	}
}
