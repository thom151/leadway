package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
)

type CutWord struct {
	Word  string `json:"word"`
	Index int    `json:"index"`
}

type openaiSmartResponse struct {
	FullScript string    `json:"full_script"`
	CutWords   []CutWord `json:"cut_words"`
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

func getResponse(c *openai.Client, thread_id, run_id string) (openaiSmartResponse, error) {

	var smartResp openaiSmartResponse

	for {
		run, err := c.RetrieveRun(context.Background(), thread_id, run_id)
		if err != nil {
			return openaiSmartResponse{}, err
		}

		if run.Status == "completed" {
			messagesList, err := c.ListMessage(context.Background(), thread_id, nil, nil, nil, nil, nil)
			if err != nil {
				return openaiSmartResponse{}, err
			}

			var assistantMsg openai.Message
			for _, msg := range messagesList.Messages {
				if msg.Role == "assistant" {
					assistantMsg = msg
					break
				}
			}

			if len(assistantMsg.Content) == 0 {
				return openaiSmartResponse{}, fmt.Errorf("No assistant content")
			}

			if assistantMsg.Content[0].Type != "text" {
				return openaiSmartResponse{}, fmt.Errorf("assistant message content is not text")
			}

			message := assistantMsg.Content[0].Text.Value

			err = json.Unmarshal([]byte(message), &smartResp)
			if err != nil {
				return openaiSmartResponse{}, err
			}
			return smartResp, nil
		}
	}
}
