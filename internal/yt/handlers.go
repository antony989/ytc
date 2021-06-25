package yt

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/kurosaki/l1/internal/models"
	"gorm.io/gorm"
)

func Run(db *gorm.DB, url string) {
	// opts := append(chromedp.DefaultExecAllocatorOptions[:]) // chromedp.DisableGPU,
	// chromedp.Flag("headless", true),

	// opts := append(chromedp.DefaultExecAllocatorOptions[:],
	// 	chromedp.DisableGPU,
	// 	chromedp.Flag("headless", false),
	// )

	ctx1, cancel1 := chromedp.NewRemoteAllocator(context.Background(), "http://chromedp:9222")
	// ctx1, cancel1 := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel1()
	ctx, cancel := chromedp.NewContext(ctx1)
	defer cancel()

	videoIdRegex := regexp.MustCompile(`https.+?\.?v=([^\s"\'<>?&]+)`)
	videoId := videoIdRegex.FindStringSubmatch(url)[1]

	chromedp.ListenTarget(ctx, func(event interface{}) {
		switch ev := event.(type) {
		case *network.EventResponseReceived:
			if ev.Type != "XHR" {
				return
			}
			match, _ := regexp.MatchString(`youtube.com\/comment\_service\_ajax`, ev.Response.URL)
			if match {
				go func() {
					comment := getComment(ctx, chromedp.FromContext(ctx), ev, videoId)
					for idx := range comment {
						db.Create(&comment[idx])
					}
				}()
			}
		}
	})

	err := chromedp.Run(
		ctx,
		network.Enable(),
		chromedp.Navigate(url),
		chromedp.Sleep(time.Second*5),
		chromedp.ActionFunc(func(ctx context.Context) error {
			videoInfo := getVideoInfo(ctx, videoId)
			db.Create(&videoInfo)
			return nil
		}),
	)

	if err != nil {
		log.Println(err)
	}

	height, _ := getScrollHeight(ctx)
	var currentHeight int

	for {
		err = chromedp.Run(
			ctx,
			tasks(url),
			chromedp.ActionFunc(func(c context.Context) error {
				_, exp, err := runtime.Evaluate(`window.scrollTo(0, document.documentElement.scrollHeight)`).Do(c)
				if err != nil {
					return err
				}
				if exp != nil {
					return exp
				}
				return nil
			}),
			chromedp.Sleep(time.Second*4),
			chromedp.Evaluate(`document.documentElement.scrollHeight`, &currentHeight),
		)
		if err != nil {
			log.Fatal(err)
		}
		if height == currentHeight {
			// wg.Done()
			break
		}
		height = currentHeight
	}
}

func getVideoInfo(ctx context.Context, videoID string) models.Video {
	var html string
	var title, description, keywords, videoThumbnail, channelName, channelImage, channelId string

	err := chromedp.Run(ctx, chromedp.OuterHTML(`head`, &html))
	HandlerError(err)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	HandlerError(err)

	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		switch name {
		case "title":
			title, _ = s.Attr("content")
		case "keywords":
			keywords, _ = s.Attr("content")
		}
	})
	doc.Find("link").Each(func(_ int, s *goquery.Selection) {
		name, _ := s.Attr("rel")
		if name == "image_src" {
			videoThumbnail, _ = s.Attr("href")
		}
	})

	err = chromedp.Run(ctx, chromedp.OuterHTML(`div#microformat`, &html))
	HandlerError(err)

	doc, err = goquery.NewDocumentFromReader(strings.NewReader(html))
	HandlerError(err)
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		id, _ := s.Attr("id")
		if id == "scriptTag" {
			contentRegex := regexp.MustCompile(`description\":\"(.+?)\"`)
			content := s.Text()
			description = contentRegex.FindStringSubmatch(content)[1]
		}
	})

	err = chromedp.Run(ctx, chromedp.OuterHTML(`div#columns`, &html))
	HandlerError(err)

	channelIdxRegex := regexp.MustCompile(`\/channel\/(.+)`)
	doc, err = goquery.NewDocumentFromReader(strings.NewReader(html))
	HandlerError(err)

	doc.Find("div#meta").Each(func(_ int, s *goquery.Selection) {
		channelImage, _ = s.Find("yt-img-shadow").Children().Attr("src")
		channelId, _ = s.Find("div.ytd-channel-name a").Attr("href")
		channelId = channelIdxRegex.FindStringSubmatch(channelId)[1]
		channelName = s.Find("div.ytd-channel-name a").Text()
	})

	VideoModel := models.Video{
		VideoID:        videoID,
		Title:          title,
		Keyword:        keywords,
		Description:    description,
		VideoThumbnail: videoThumbnail,
		ChannelName:    channelName,
		ChannelImage:   channelImage,
		ChannelId:      channelId,
	}
	return VideoModel
}

func getComment(ctx context.Context, c *chromedp.Context, ev *network.EventResponseReceived, videoId string) []models.MainComment {
	rbp := network.GetResponseBody(ev.RequestID)
	sessionTokenGet := network.GetRequestPostData(ev.RequestID)
	stRegex := regexp.MustCompile(`session.+?\=(.+)`)
	st, _ := sessionTokenGet.Do(cdp.WithExecutor(ctx, c.Target))
	sessionToken := stRegex.FindStringSubmatch(st)[1]
	body, _ := rbp.Do(cdp.WithExecutor(ctx, c.Target))
	json, _ := gabs.ParseJSON(body)

	var CommentModel []models.MainComment
	var replies [][]models.RepliesComment
	var authorChannelIds, commentIds, thumbnails, userNames, contentTexts []string
	var voteCounts []int64
	var commentDates []time.Time
	// var isModeratedElqComments []bool
	for _, child := range json.Search("response", "continuationContents", "itemSectionContinuation", "contents").Children() {
		authorChannelId := child.Path("commentThreadRenderer.comment.commentRenderer.authorEndpoint.browseEndpoint.browseId").Data().(string)
		commentId := child.Search("commentThreadRenderer", "comment", "commentRenderer", "commentId").Data().(string)

		voteCount, ok := child.Path("commentThreadRenderer.comment.commentRenderer.voteCount.simpleText").Data().(string)
		var voteCountInt int64
		if ok {
			voteCountInt, _ = strconv.ParseInt(voteCount, 10, 64)
		}
		// isModeratedElqComment := child.Search("commentThreadRenderer", "isModeratedElqComment").Data().(bool)
		thumbnail := child.Path("commentThreadRenderer.comment.commentRenderer.authorThumbnail.thumbnails.0.url").Data().(string)
		// .authorThumbnail.thumbnails.0.url
		userName := child.Search("commentThreadRenderer", "comment", "commentRenderer", "authorText", "simpleText").Data().(string)
		publishTimeText := child.Path("commentThreadRenderer.comment.commentRenderer.publishedTimeText.runs.0.text").Data().(string)
		commentDate := timeStampChanger(publishTimeText)
		contentText := child.Path("commentThreadRenderer.comment.commentRenderer.contentText.runs").Children()

		replyCount := child.Search("commentThreadRenderer", "comment", "commentRenderer", "replyCount").Data()

		if replyCount != nil {
			repliesClickParams := child.Path("commentThreadRenderer.replies.commentRepliesRenderer.continuations.0.nextContinuationData.clickTrackingParams").Data()
			repliesContinuation := child.Path("commentThreadRenderer.replies.commentRepliesRenderer.continuations.0.nextContinuationData.continuation").Data()
			repliesUri := fmt.Sprintf("https://www.youtube.com/comment_service_ajax?action_get_comment_replies=1&pbj=1&ctoken=%v&continuation=%s&type=next&itct=%v", repliesContinuation, repliesContinuation, repliesClickParams)

			replies = append(replies, repliesRequest(ev.Response.RequestHeaders, repliesUri, sessionToken, videoId, true))
		} else {
			replies = append(replies, repliesRequest(nil, "", "", "", false))
		}

		authorChannelIds = append(authorChannelIds, authorChannelId)
		commentIds = append(commentIds, commentId)
		// isModeratedElqComments = append(isModeratedElqComments, isModeratedElqComment)
		userNames = append(userNames, userName)
		thumbnails = append(thumbnails, thumbnail)
		commentDates = append(commentDates, commentDate)
		voteCounts = append(voteCounts, voteCountInt)
		contentTexts = append(contentTexts, contentCombiner(contentText))
	}

	for k, v := range authorChannelIds {
		CommentModel = append(
			CommentModel,
			models.MainComment{
				VideoId:   videoId,
				ChannelId: v,
				CommentId: commentIds[k],
				UserName:  userNames[k],
				Content:   contentTexts[k],
				Thumbnail: thumbnails[k],
				CreatedAt: commentDates[k],
				UpdatedAt: time.Now(),
				Replies:   replies[k],
				VoteCount: voteCounts[k],
			},
		)
	}

	return CommentModel
}

func repliesRequest(h network.Headers, xhrUrl, xsrf, video_id string, isReplies bool) []models.RepliesComment {
	var RepliesCommentModel []models.RepliesComment
	if isReplies {
		data := url.Values{}
		xsrf, _ = url.QueryUnescape(xsrf)
		data.Set("session_token", xsrf)
		s := data.Encode()
		client := &http.Client{}
		req, err := http.NewRequest("POST", xhrUrl, strings.NewReader(s))
		for k, v := range h {
			if k == ":method" {
				k = "method"
			} else if k == ":scheme" {
				k = "scheme"
			} else if k == ":authority" {
				k = "authority"
			} else if k == ":path" {
				k = "path"
			}
			req.Header.Add(k, v.(string))
		}
		if err != nil {
			log.Fatalln(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalln(err)
		}
		defer resp.Body.Close()
		// content-encoding br[brotli]
		reader := brotli.NewReader(resp.Body)
		body, _ := ioutil.ReadAll(reader)
		Getjson, _ := gabs.ParseJSON(body)

		//contentTexts
		var authorChannelIds, commentIds, thumbnails, userNames, contentTexts []string
		var voteCounts []int64
		var commentDates []time.Time

		for _, child := range Getjson.Path("1.response.continuationContents.commentRepliesContinuation.contents").Children() {
			authorChannelId := child.Path("commentRenderer.authorEndpoint.browseEndpoint.browseId").Data().(string)
			commentId := child.Search("commentRenderer", "commentId").Data().(string)
			userName := child.Search("commentRenderer", "authorText", "simpleText").Data().(string)
			contentText := child.Path("commentRenderer.contentText.runs").Children()
			voteCount, ok := child.Path("commentRenderer.voteCount.simpleText").Data().(string)
			var voteCountInt int64
			if ok {
				voteCountInt, _ = strconv.ParseInt(voteCount, 10, 64)
			}
			thumbnail := child.Path("commentRenderer.authorThumbnail.thumbnails.0.url").Data().(string)
			publishTimeText := child.Path("commentRenderer.publishedTimeText.runs.0.text").Data().(string)
			commentDate := timeStampChanger(publishTimeText)
			authorChannelIds = append(authorChannelIds, authorChannelId)
			commentIds = append(commentIds, commentId)
			voteCounts = append(voteCounts, voteCountInt)
			userNames = append(userNames, userName)
			contentTexts = append(contentTexts, contentCombiner(contentText))
			thumbnails = append(thumbnails, thumbnail)
			commentDates = append(commentDates, commentDate)
		}

		for k, v := range authorChannelIds {
			RepliesCommentModel = append(
				RepliesCommentModel,
				models.RepliesComment{
					VideoId:        video_id,
					ChannelId:      v,
					ReplyCommentId: commentIds[k],
					UserName:       userNames[k],
					Content:        contentTexts[k],
					Thumbnail:      thumbnails[k],
					CreatedAt:      commentDates[k],
					UpdatedAt:      time.Now(),
					VoteCount:      voteCounts[k],
				},
			)
		}

		MoreRepliesrepliesClickParams := Getjson.Path("1.response.continuationContents.commentRepliesContinuation.continuations.0.nextContinuationData.clickTrackingParams").Data()
		MoreRepliesrepliesContinuation := Getjson.Path("1.response.continuationContents.commentRepliesContinuation.continuations.0.nextContinuationData.continuation").Data()
		if MoreRepliesrepliesClickParams != nil {
			repliesUri := fmt.Sprintf("https://www.youtube.com/comment_service_ajax?action_get_comment_replies=1&pbj=1&ctoken=%v&continuation=%s&type=next&itct=%v", MoreRepliesrepliesContinuation, MoreRepliesrepliesContinuation, MoreRepliesrepliesClickParams)
			sub := repliesRequest(h, repliesUri, xsrf, video_id, true)
			for _, v := range sub {
				RepliesCommentModel = append(
					RepliesCommentModel,
					models.RepliesComment{
						VideoId:        v.VideoId,
						ChannelId:      v.ChannelId,
						ReplyCommentId: v.ReplyCommentId,
						UserName:       v.UserName,
						Content:        v.Content,
						Thumbnail:      v.Thumbnail,
						CreatedAt:      v.CreatedAt,
						UpdatedAt:      v.UpdatedAt,
						VoteCount:      v.VoteCount,
					},
				)
			}
		} else {
			repliesRequest(nil, "", "", "", false)
		}
		return RepliesCommentModel
	} else {
		return RepliesCommentModel
	}
}

func HandlerError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func tasks(url string) chromedp.Tasks {
	return chromedp.Tasks{
		network.Enable(),
		chromedp.Sleep(time.Second * 6),
	}
}

func getScrollHeight(ctx context.Context) (int, error) {
	var height int
	err := chromedp.Run(ctx, chromedp.Evaluate(`document.documentElement.scrollHeight`, &height))
	return height, err
}

func contentCombiner(contentText []*gabs.Container) string {
	var content string
	for _, v := range contentText {
		s := strings.Trim(v.Search("text").String(), "\"")
		s = strings.TrimSuffix(s, "\n")
		content += s
	}
	return content
}

func timeStampChanger(timeDate string) time.Time {
	dateRegex := regexp.MustCompile(`([0-9]+)+(.+ago)`)
	commentTextdate := dateRegex.FindStringSubmatch(timeDate)
	dateParser := commentTextdate[1]
	dateString := commentTextdate[2]
	dateInt, _ := strconv.Atoi(dateParser)
	var commentDate time.Time
	switch dateString {
	case "minute ago":
		commentDate = time.Now().Add(time.Minute * time.Duration(-dateInt))

	case "minutes ago":
		commentDate = time.Now().Add(time.Minute * time.Duration(-dateInt))

	case "hour ago":
		commentDate = time.Now().Add(time.Hour * time.Duration(-dateInt))

	case "hours ago":
		commentDate = time.Now().Add(time.Hour * time.Duration(-dateInt))

	case "day ago":
		commentDate = time.Now().Add((time.Hour * 24) * time.Duration(-dateInt))

	case "days ago":
		commentDate = time.Now().Add((time.Hour * 24) * time.Duration(-dateInt))

	case "week ago":
		commentDate = time.Now().Add((time.Hour * 24 * 7) * time.Duration(-dateInt))

	case "weeks ago":
		commentDate = time.Now().Add((time.Hour * 24 * 7) * time.Duration(-dateInt))

	case "month ago":
		commentDate = time.Now().Add((time.Hour * 30 * 24) * time.Duration(-dateInt))

	case "months ago":
		commentDate = time.Now().Add((time.Hour * 30 * 24) * time.Duration(-dateInt))

	case "year ago":
		commentDate = time.Now().Add((time.Hour * 24 * 365) * time.Duration(-dateInt))

	case "years ago":
		commentDate = time.Now().Add((time.Hour * 24 * 365) * time.Duration(-dateInt))

	default:
		commentDate = time.Now().Add(time.Minute * time.Duration(-dateInt))
	}
	return commentDate
}
