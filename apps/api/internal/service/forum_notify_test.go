package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/utils"
)

/* ------------------------------------------------------------------ *
 * #222：回复通知（ADR 0007）
 * ------------------------------------------------------------------ */

// notifyRecordingMailer 记录每次通知调用的收件人，便于断言「发给了谁、发了几封」。
type notifyRecordingMailer struct {
	mu      sync.Mutex
	calls   []string // 收件人邮箱
	err     error
	blockOn chan struct{} // 非 nil 时阻塞发送，用于测队列满与停机
}

func (m *notifyRecordingMailer) SendQaReplyNotice(_ context.Context, author *domain.User, _, _, _ string) error {
	if m.blockOn != nil {
		<-m.blockOn
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, author.Detail.Email)
	return m.err
}

func (m *notifyRecordingMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// notifyUserReader 按 id 返回预置用户。
type notifyUserReader struct {
	users map[int]*domain.User
	err   error
}

func (r *notifyUserReader) GetByID(_ context.Context, id int) (*domain.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.users[id], nil
}

func notifyPost(authorID int) *domain.ForumPost {
	return &domain.ForumPost{ID: 3, AuthorID: authorID, Title: "如何准备面试", Scope: domain.ForumScopeRecruitment}
}

func userWithMail(id int, name, email string) *domain.User {
	u := &domain.User{ID: id, Name: name}
	u.Detail.Email = email
	return u
}

func TestNotifyForumReply_SkipsSelfReply(t *testing.T) {
	// 自己回复自己的帖子：不发（AC：自己回自己不发）。
	mailer := &notifyRecordingMailer{}
	q := NewForumNotifyQueue(mailer)
	uc := NewForumUseCase(nil, nil).WithDirectionFallback(&notifyUserReader{}, nil).WithReplyNotify(q)

	post := notifyPost(5)
	reply := &domain.ForumReply{ID: 1, PostID: 3, AuthorID: 5, Body: "补充一下"}
	uc.NotifyForumReply(context.Background(), post, reply, "我自己")

	if _, _, _ = q.Stats(); len(q.queue) != 0 {
		t.Fatalf("自己回自己不应入队，队列长度 %d", len(q.queue))
	}
}

func TestNotifyForumReply_SkipsEmptyEmail(t *testing.T) {
	// 作者邮箱为空：跳过并记录，不视为失败（AC 明确要求）。
	mailer := &notifyRecordingMailer{}
	q := NewForumNotifyQueue(mailer)
	uc := NewForumUseCase(nil, nil).
		WithDirectionFallback(&notifyUserReader{users: map[int]*domain.User{
			5: userWithMail(5, "张三", ""), // 空邮箱
		}}, nil).
		WithReplyNotify(q)

	uc.NotifyForumReply(context.Background(), notifyPost(5), &domain.ForumReply{ID: 1, AuthorID: 9}, "李四")

	if len(q.queue) != 0 {
		t.Fatalf("空邮箱不应入队，队列长度 %d", len(q.queue))
	}
}

func TestNotifyForumReply_EnqueuesForOtherAuthor(t *testing.T) {
	mailer := &notifyRecordingMailer{}
	q := NewForumNotifyQueue(mailer)
	q.Start()
	defer q.Stop()

	uc := NewForumUseCase(nil, nil).
		WithDirectionFallback(&notifyUserReader{users: map[int]*domain.User{
			5: userWithMail(5, "张三", "zhang@example.com"),
		}}, nil).
		WithReplyNotify(q)

	uc.NotifyForumReply(context.Background(), notifyPost(5), &domain.ForumReply{ID: 1, AuthorID: 9}, "李四")

	// 等 worker 消费完（带超时，避免测试挂住）。
	deadline := time.Now().Add(2 * time.Second)
	for mailer.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if mailer.count() != 1 {
		t.Fatalf("应发送 1 封通知，实际 %d", mailer.count())
	}
}

func TestNotifyForumReply_UserLookupFailureIsLogged(t *testing.T) {
	// 取作者失败只降级，不 panic、不入队。
	q := NewForumNotifyQueue(&notifyRecordingMailer{})
	uc := NewForumUseCase(nil, nil).
		WithDirectionFallback(&notifyUserReader{err: errors.New("db down")}, nil).
		WithReplyNotify(q)

	uc.NotifyForumReply(context.Background(), notifyPost(5), &domain.ForumReply{ID: 1, AuthorID: 9}, "李四")

	if len(q.queue) != 0 {
		t.Fatalf("取作者失败不应入队")
	}
}

/* ------------------------------------------------------------------ *
 * 队列行为
 * ------------------------------------------------------------------ */

func TestForumNotifyQueue_DropsWhenFull(t *testing.T) {
	// 队列满时非阻塞丢弃并记录（AC：队列满丢弃）。
	// 不 Start worker，让队列真正堆满。
	release := make(chan struct{})
	mailer := &notifyRecordingMailer{blockOn: release}
	q := NewForumNotifyQueue(mailer)

	// 直接填满内部 channel（不通过 Enqueue，避免日志刷屏）。
	author := userWithMail(5, "张三", "a@example.com")
	for i := 0; i < forumNotifyQueueSize; i++ {
		q.queue <- forumNotifyTask{author: author, postTitle: "t"}
	}

	ok := q.Enqueue(forumNotifyTask{author: author, postTitle: "overflow"})
	if ok {
		t.Fatal("队列已满时 Enqueue 应返回 false")
	}
	_, dropped, _ := q.Stats()
	if dropped != 1 {
		t.Fatalf("应记录 1 次丢弃，实际 %d", dropped)
	}
}

func TestForumNotifyQueue_SendFailureDoesNotPanic(t *testing.T) {
	// 发送失败只记日志（AC：通知失败不回滚回复）。
	mailer := &notifyRecordingMailer{err: errors.New("smtp down")}
	q := NewForumNotifyQueue(mailer)
	q.Start()
	defer q.Stop()

	q.Enqueue(forumNotifyTask{author: userWithMail(5, "张三", "a@example.com"), postTitle: "t"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, _, failed := q.Stats()
		if failed == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("发送失败应被计入 failed 计数")
}

func TestForumNotifyQueue_StopDrainsQueued(t *testing.T) {
	// 停机时已入队的尽力发完（AC：停机时最多等 5 秒尽力发完）。
	mailer := &notifyRecordingMailer{}
	q := NewForumNotifyQueue(mailer)
	// 不 Start，直接入队再 Stop：drain 只由 Start 的 goroutine 触发，
	// 所以这里启动后再入队。
	q.Start()
	for i := 0; i < 3; i++ {
		q.Enqueue(forumNotifyTask{author: userWithMail(5, "张三", "a@example.com"), postTitle: "t"})
	}
	q.Stop()

	if mailer.count() != 3 {
		t.Fatalf("停机前已入队的 3 封应尽力发完，实际 %d", mailer.count())
	}
}

/* ------------------------------------------------------------------ *
 * 内置兜底文案（AC：兜底文案渲染成功）
 * ------------------------------------------------------------------ */

func TestBuiltInQaReplyMail_RendersWithAutoVarsOnly(t *testing.T) {
	// 内置文案只引用 render 自动预填的变量，务必能渲染成功（ADR 0007 决策 2）。
	model := builtInQaReplyMail()

	// 只给 render 会自动预填的那几个变量。
	vars := utils.TemplateVars{
		"name":  "张三",
		"email": "zhang@example.com",
		"class": "软件工程230301",
	}

	title, err := utils.RenderTemplate(model.Title, vars)
	if err != nil {
		t.Fatalf("内置兜底标题应能渲染，得到错误：%v", err)
	}
	content, err := utils.RenderTemplate(model.MailModel, vars)
	if err != nil {
		t.Fatalf("内置兜底正文应能渲染，得到错误：%v", err)
	}
	if !strings.Contains(title, "回复") {
		t.Fatalf("标题应提示有新回复，实际 %q", title)
	}
	if !strings.Contains(content, "张三") {
		t.Fatalf("正文应包含收件人姓名，实际 %q", content)
	}
	// 只有预填变量时也必须成功：说明没有引用 post_title/reply_excerpt 等额外变量。
	if strings.Contains(content, "{{") || strings.Contains(content, "}}") {
		t.Fatalf("正文不应残留未渲染的占位符：%q", content)
	}
}

func TestTruncateRunes_DoesNotBreakCJK(t *testing.T) {
	// 按字符截断，不切坏中文（ADR 0007 决策 8）。
	long := strings.Repeat("中", 300)
	got := truncateRunes(long, 200)
	r := []rune(got)
	// 200 个字符 + 1 个省略号
	if len(r) != 201 {
		t.Fatalf("应截成 200 字符 + 省略号，实际 %d", len(r))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("截断后应有省略号，实际 %q", got[len(got)-3:])
	}

	short := "短内容"
	if got := truncateRunes(short, 200); got != short {
		t.Fatalf("未超限应原样返回，实际 %q", got)
	}
}

// TestUpdateReply_DoesNotNotify 编辑回复不触发通知（AC：编辑回复不触发，ADR 0007 决策 10）。
//
// 编辑是修正笔误，不是新的交流事件。再发一次会让「有人回你」这个信号贬值。
func TestUpdateReply_DoesNotNotify(t *testing.T) {
	mailer := &notifyRecordingMailer{}
	q := NewForumNotifyQueue(mailer)
	q.Start()
	defer q.Stop()

	repo := &stubForumWriteRepo{
		reply: &domain.ForumReply{ID: 1, PostID: 3, AuthorID: 9, Body: "原文"},
	}
	uc := NewForumUseCase(repo, &stubAdminDirectory{ids: []int{9}}).
		WithDirectionFallback(&notifyUserReader{users: map[int]*domain.User{
			5: userWithMail(5, "张三", "zhang@example.com"),
		}}, nil).
		WithReplyNotify(q)

	if _, err := uc.UpdateReply(context.Background(), 9, 1, CreateForumReplyInput{Body: "改后的正文"}); err != nil {
		t.Fatalf("编辑回复应成功，得到 %v", err)
	}
	// 给 worker 一点时间；如果错误地入队了，这里就会捕捉到。
	time.Sleep(50 * time.Millisecond)
	if mailer.count() != 0 {
		t.Fatalf("编辑回复不应发送通知，实际发了 %d 封", mailer.count())
	}
}
