package handlers

import (
	"context"
	"fmt"
	"os"
	"start-feishubot/initialization"
	"start-feishubot/logger"
	"start-feishubot/services"
	"start-feishubot/services/openai"
	"start-feishubot/utils"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type VisionAction struct { /*图片推理*/
}

func (*VisionAction) Execute(a *ActionInfo) bool {
	check := AzureModeCheck(a)
	if !check {
		return true
	}
	// 开启图片创作模式
	if _, foundPic := utils.EitherTrimEqual(a.info.qParsed,
		"/vision", "图片推理"); foundPic {
		a.handler.sessionCache.Clear(*a.info.sessionId)
		a.handler.sessionCache.SetMode(*a.info.sessionId,
			services.ModeVision)
		a.handler.sessionCache.SetVisionDetail(*a.info.sessionId,
			services.VisionDetailHigh)
		sendVisionInstructionCard(*a.ctx, a.info.sessionId,
			a.info.msgId)
		return false
	}

	mode := a.handler.sessionCache.GetMode(*a.info.sessionId)
	fmt.Println("a.info.msgType: ", a.info.msgType)
	logger.Debug("MODE:", mode)
	// 收到一张图片,且不在图片推理模式下, 提醒是否切换到图片推理模式
	if a.info.msgType == "image" && mode != services.ModeVision {
		sendVisionModeCheckCard(*a.ctx, a.info.sessionId, a.info.msgId)
		return false
	}

	if a.info.msgType == "image" && mode == services.ModeVision {
		//保存图片
		imageKey := a.info.imageKey
		//fmt.Printf("fileKey: %s \n", imageKey)
		msgId := a.info.msgId
		//fmt.Println("msgId: ", *msgId)
		req := larkim.NewGetMessageResourceReqBuilder().MessageId(
			*msgId).FileKey(imageKey).Type("image").Build()
		resp, err := initialization.GetLarkClient().Im.MessageResource.Get(context.Background(), req)
		fmt.Println(resp, err)
		if err != nil {
			//fmt.Println(err)
			replyMsg(*a.ctx, fmt.Sprintf("🤖️：图片下载失败，请稍后再试～\n 错误信息: %v", err),
				a.info.msgId)
			return false
		}

		f := fmt.Sprintf("%s.png", imageKey)
		fmt.Println(f)
		resp.WriteFile(f)
		defer os.Remove(f)
		//resolution := a.handler.sessionCache.GetPicResolution(*a.
		//	info.sessionId)

		base64, err := openai.GetBase64FromImage(f)
		if err != nil {
			replyMsg(*a.ctx, fmt.Sprintf("🤖️：图片下载失败，请稍后再试～\n 错误信息: %v", err),
				a.info.msgId)
			return false
		}
		//
		var msg []openai.VisionMessages
		detail := a.handler.sessionCache.GetVisionDetail(*a.info.sessionId)
		// 如果没有提示词，默认模拟ChatGPT
		msg = append(msg, openai.VisionMessages{
			Role: "user", Content: []openai.ContentType{
				{
					Type: "image", ImageURL: openai.
						ImageURL{URL: base64, Detail: detail},
				},
			},
		})
		// get ai mode as temperature
		fmt.Println("msg: ", msg)
		completions, err := a.handler.gpt.GetVisionInfo(msg)
		if err != nil {
			replyMsg(*a.ctx, fmt.Sprintf(
				"🤖️：消息机器人摆烂了，请稍后再试～\n错误信息: %v", err), a.info.msgId)
			return false
		}
		msg = append(msg, completions)
		a.handler.sessionCache.SetMsg(*a.info.sessionId, msg)

		////图片校验
		//err = openai.VerifyPngs([]string{f})
		//if err != nil {
		//	replyMsg(*a.ctx, fmt.Sprintf("🤖️：无法解析图片，请发送原图并尝试重新操作～"),
		//		a.info.msgId)
		//	return false
		//}
		//bs64, err := a.handler.gpt.GenerateOneImageVariation(f, resolution)
		//if err != nil {
		//	replyMsg(*a.ctx, fmt.Sprintf(
		//		"🤖️：图片生成失败，请稍后再试～\n错误信息: %v", err), a.info.msgId)
		//	return false
		//}
		replayImagePlainByBase64(*a.ctx, base64, a.info.msgId)
		return false

	}

	// 生成图片
	if mode == services.ModePicCreate {
		resolution := a.handler.sessionCache.GetPicResolution(*a.
			info.sessionId)
		style := a.handler.sessionCache.GetPicStyle(*a.
			info.sessionId)
		bs64, err := a.handler.gpt.GenerateOneImage(a.info.qParsed,
			resolution, style)
		if err != nil {
			replyMsg(*a.ctx, fmt.Sprintf(
				"🤖️：图片生成失败，请稍后再试～\n错误信息: %v", err), a.info.msgId)
			return false
		}
		replayImageCardByBase64(*a.ctx, bs64, a.info.msgId, a.info.sessionId,
			a.info.qParsed)
		return false
	}

	return true
}
