package chatgpt_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/mailroom/services/external/openai/chatgpt"
	"github.com/stretchr/testify/assert"
)

const (
	baseURL = "https://chatgpt.com.br"
	apiKey  = "DUMMY_API_KEY"
)

func TestCreateChatCompletion(t *testing.T) {
	defer httpx.SetRequestor(httpx.DefaultRequestor)
	httpx.SetRequestor(httpx.NewMockRequestor(map[string][]httpx.MockResponse{
		fmt.Sprintf("%s/v1/chat/completions", baseURL): {
			httpx.NewMockResponse(400, nil, `{
				"error": {
					"message": "dummy error message",
					"type": "dummy error type"
				}
			}`),
			httpx.NewMockResponse(200, nil, `{
				"id": "chatcmpl-7IfBIQsTVKbwOiHPgcrpthaCn7K1t",
				"object": "chat.completion",
				"created":1684682560,
				"model":"gpt-3.5-turbo-0301",
				"usage":{
					"prompt_tokens":26,
					"completion_tokens":8,
					"total_tokens":34
				},
				"choices":[
					{
						"message":{
							"role":"assistant",
							"content":"This is a test!"
						},
						"finish_reason":"stop",
						"index":0
					}
				]
			}`),
		},
	}))

	client := chatgpt.NewClient(http.DefaultClient, nil, baseURL, apiKey)

	messages := []chatgpt.ChatCompletionMessage{
		{
			Role:    chatgpt.ChatMessageRoleUser,
			Content: "Say this is a test!",
		},
	}

	data := chatgpt.NewChatCompletionRequest(messages)

	_, _, err := client.CreateChatCompletion(data)
	assert.EqualError(t, err, "message:dummy error message. type:dummy error type")

	cmsg, trace, err := client.CreateChatCompletion(data)
	assert.NoError(t, err)
	assert.Equal(t, "chatcmpl-7IfBIQsTVKbwOiHPgcrpthaCn7K1t", cmsg.ID)
	assert.Equal(t, "This is a test!", cmsg.Choices[0].Message.Content)
	assert.Equal(t, "HTTP/1.0 200 OK\r\nContent-Length: 419\r\n\r\n", string(trace.ResponseTrace))
}
