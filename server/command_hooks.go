package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
)

const (
	commandTriggerDanmaku = "danmaku"
)

func (p *Plugin) getAutocompleteIconData(icon_name string) string {
	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		p.API.LogError("Couldn't get bundle path", "error", err)
		return ""
	}

	icon, err := os.ReadFile(filepath.Join(bundlePath, "assets", icon_name))
	if err != nil {
		p.API.LogError("Failed to open icon", "error", err)
		return ""
	}

	return fmt.Sprintf("data:image/svg+xml;base64,%s", base64.StdEncoding.EncodeToString(icon))
}

func (p *Plugin) registerCommands() error {
	if err := p.API.RegisterCommand(&model.Command{
		Trigger:              commandTriggerDanmaku,
		AutoComplete:         true,
		AutoCompleteHint:     "",
		AutoCompleteDesc:     "向所有人发送弹幕",
		AutocompleteIconData: p.getAutocompleteIconData("danmaku.svg"),
	}); err != nil {
		return errors.Wrapf(err, "failed to register %s command", commandTriggerDanmaku)
	}
	return nil
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	trigger := strings.TrimPrefix(strings.Fields(args.Command)[0], "/")
	switch trigger {
	case commandTriggerDanmaku:
		return p.executeCommandDanmaku(args), nil

	default:
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unknown command: " + args.Command),
		}, nil
	}
}

type BroadcastRequest struct {
	RoomID     string `json:"room_id"`
	CourseName string `json:"course_name"`
	PageNumber string `json:"page_number"`
	Content    string `json:"content"`
	Type       string `json:"type"`
	AvatarURL  string `json:"avatar_url"`
}

func sendBroadcast(p *Plugin, text string) (map[string]string, error) {
	// 创建请求体
	requestBody := BroadcastRequest{
		RoomID:     "",
		CourseName: "",
		PageNumber: "-1",
		Content:    text,
		Type:       "command",
		AvatarURL:  "https://vip.123pan.cn/1841937928/11391818",
	}

	// 将请求体编码为 JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("编码请求体失败: %v", err)
	}

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 10 * time.Second, // 设置超时时间
	}

	// 创建请求
	configuration := p.getConfiguration()
	req, err := http.NewRequest("POST", configuration.DanmakuUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("广播失败: %s", string(body))
	}

	// 返回成功响应
	return map[string]string{
		"response_type": "ephemeral",
		"text":          "发送成功",
	}, nil
}

func (p *Plugin) executeCommandDanmaku(args *model.CommandArgs) *model.CommandResponse {
	// 使用 strings.Fields 分割字符串
	fields := strings.Fields(args.Command)

	// 声明 remainingArgs 在外部作用域
	var remainingArgs string

	// 如果字段数大于 1，则获取命令之后的内容
	if len(fields) > 1 {
		// 将剩余部分重新组合为字符串
		remainingArgs = strings.Join(fields[1:], " ")
	}

	var post *model.Post

	// 检查内容是否为空
	if remainingArgs == "" {
		post = &model.Post{
			ChannelId: args.ChannelId,
			Message:   "发送内容不能为空",
		}
	} else {
		// 发送弹幕
		sendBroadcast(p, remainingArgs)

		post = &model.Post{
			ChannelId: args.ChannelId,
			Message:   "弹幕发送成功🎉",
		}
	}

	_ = p.API.SendEphemeralPost(args.UserId, post)
	return &model.CommandResponse{}
}
