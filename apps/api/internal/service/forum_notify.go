package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/IOT-devstudio/iot-pillot/apps/api/internal/domain"
)

// qaReplyMailType 回复通知邮件的模板类型（ADR 0007 决策 1）。
//
// 两个内容域共用一套：通知正文对 scope 不敏感（都是「你的帖子有新回复」），
// 配两条模板只会因为漏配一边而少一类通知。
const qaReplyMailType = "qa_reply"

// forumNotifyQueueSize 通知队列容量（ADR 0007 决策 3）。
//
// 有界是有意的：无界队列只是把「无限起 goroutine」换成「无限吃内存」，
// SMTP 卡死时同样拖垮进程。
const forumNotifyQueueSize = 256

// forumNotifyStopWait Stop 时等已入队任务发完的上限（ADR 0007 决策 3）。
//
// 发版不能因为 SMTP 卡住而拖长退出时间；5 秒是「尽力」而非「保证」。
const forumNotifyStopWait = 5 * time.Second

// forumNotifySendTimeout 单封通知的发送上限。
//
// SendEmail 内部已有 10 秒上限；这里再包一层是为了让整个任务
// （取模板 + 取用户 + 渲染 + 投递）都有界，避免某项卡住拖住 worker。
const forumNotifySendTimeout = 30 * time.Second

// ForumNotifyMailer 发送回复通知所需的窄接口。
//
// 只要一个方法：队列不关心模板选择与渲染细节，那些都在 MailUseCase 里。
// 用窄接口而不是直接依赖 *MailUseCase，测试里就不必起整套邮件依赖。
type ForumNotifyMailer interface {
	// SendQaReplyNotice 给帖子作者发「你的帖子有新回复」通知。
	// 收件人没有邮箱时返回错误（由调用方按「跳过并记录」处理）。
	SendQaReplyNotice(ctx context.Context, author *domain.User, postTitle string, replyBody string, replierName string) error
}

// ForumUserReader 取用户资料（需要 Detail.Email 与 Name）。
type ForumUserReader interface {
	GetByID(ctx context.Context, id int) (*domain.User, error)
}

// forumNotifyTask 一条待投递的通知。
//
// 在入队时就把渲染需要的素材取好（而不是 worker 里再查库）：
// 帖子可能在这几秒内被删，延迟查会拿到 nil 或已软删的行，
// 而通知说的是「刚刚有人回复了你」——那一刻的事实已经确定了。
type forumNotifyTask struct {
	postTitle   string
	replyBody   string
	replierName string
	author      *domain.User
}

// ForumNotifyQueue 回复通知的有界队列 + 单 worker（ADR 0007 决策 3）。
//
// 为什么不裸 go func()：无并发上限；进程重启或发版时在途那封直接丢，且丢了没人知道。
// 为什么不同步发：SMTP 半死不活时用户点「回复」要转 10 秒，且被卡的 goroutine 还占着连接。
type ForumNotifyQueue struct {
	mailer ForumNotifyMailer
	queue  chan forumNotifyTask

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}

	// sent / dropped / failed 仅供测试与日志观察，不做指标采集（ADR 0007 决策 11）。
	mu      sync.Mutex
	sent    int
	dropped int
	failed  int
}

// NewForumNotifyQueue 构造队列（不启动）。
func NewForumNotifyQueue(mailer ForumNotifyMailer) *ForumNotifyQueue {
	return &ForumNotifyQueue{
		mailer: mailer,
		queue:  make(chan forumNotifyTask, forumNotifyQueueSize),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start 启动单 worker。重复调用安全但无意义；生命周期由 Router 控制。
func (q *ForumNotifyQueue) Start() {
	go func() {
		defer close(q.done)
		for {
			select {
			case task := <-q.queue:
				q.deliver(task)
			case <-q.stop:
				// 停机：把已入队的尽力发完，但不超过 forumNotifyStopWait。
				q.drain()
				return
			}
		}
	}()
}

// Stop 停止接收并把已入队的尽力发完，最多等 forumNotifyStopWait。
//
// 为什么不直接丢弃队列：发版瞬间那批通知会静默消失，日志里看不出丢了几条。
// 为什么不无限等：SMTP 卡死会让发版永远退不出去。
func (q *ForumNotifyQueue) Stop() {
	q.stopOnce.Do(func() {
		close(q.stop)
		select {
		case <-q.done:
		case <-time.After(forumNotifyStopWait):
			log.Printf("[forum] 通知队列停机超时（%s），剩余未发任务被放弃", forumNotifyStopWait)
		}
	})
}

// drain 在停机时尽量把剩余任务发完。
//
// 用带截止的循环而不是「for range q.queue」：后者要等 channel 关闭，
// 而我们没有关闭它（关闭会让还在入队的 Enqueue panic）。
func (q *ForumNotifyQueue) drain() {
	deadline := time.After(forumNotifyStopWait)
	for {
		select {
		case task := <-q.queue:
			q.deliver(task)
		case <-deadline:
			return
		default:
			return
		}
	}
}

// deliver 发送一封通知，失败只记日志（ADR 0007 决策 4/11：不回滚回复）。
func (q *ForumNotifyQueue) deliver(task forumNotifyTask) {
	// 独立 context：队列的生命周期与发起回复的那个请求无关。
	ctx, cancel := context.WithTimeout(context.Background(), forumNotifySendTimeout)
	defer cancel()

	err := q.mailer.SendQaReplyNotice(ctx, task.author, task.postTitle, task.replyBody, task.replierName)
	if err != nil {
		q.mu.Lock()
		q.failed++
		q.mu.Unlock()
		// 前缀 [forum] 便于过滤（ADR 0007 决策 11）。
		log.Printf("[forum] 回复通知发送失败（不影响回复本身）：收件人 user_id=%d，标题=%q，err=%v",
			task.author.ID, task.postTitle, err)
		return
	}
	q.mu.Lock()
	q.sent++
	q.mu.Unlock()
}

// Enqueue 非阻塞投递一条通知。
//
// 队列满时**丢弃并记日志**（ADR 0007 决策 3）：回复绝不能因为通知队列满
// 而变慢或失败。返回是否入队成功，便于测试与调用方观察。
func (q *ForumNotifyQueue) Enqueue(task forumNotifyTask) bool {
	select {
	case q.queue <- task:
		return true
	default:
		q.mu.Lock()
		q.dropped++
		q.mu.Unlock()
		log.Printf("[forum] 回复通知队列已满，丢弃一条通知（收件人 user_id=%d，标题=%q）",
			task.author.ID, task.postTitle)
		return false
	}
}

// Stats 返回 sent / dropped / failed 计数（测试与诊断用）。
func (q *ForumNotifyQueue) Stats() (sent, dropped, failed int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.sent, q.dropped, q.failed
}

/* ------------------------------------------------------------------ *
 * 触发点：创建回复之后
 * ------------------------------------------------------------------ */

// NotifyForumReply 在「新建回复」之后决定是否发通知（ADR 0007 决策 5/10）。
//
// 规则：
//   - 自己回复自己的帖子 → 不发（自己收到自己的通知是纯噪音）；
//   - 作者邮箱为空 → 不发，记日志跳过（不为没有邮箱的账号编造收件地址）；
//   - 其它情况 → 入队。
//
// 本方法**不返回错误**：调用方（CreateReply）不该因为通知出问题而失败。
// 所有异常都降级成日志。
func (uc *ForumUseCase) NotifyForumReply(ctx context.Context, post *domain.ForumPost, reply *domain.ForumReply, replierName string) {
	if uc.notify == nil || uc.users == nil {
		return
	}
	// 自己回复自己：跳过。
	if post.AuthorID == reply.AuthorID {
		return
	}

	author, err := uc.users.GetByID(ctx, post.AuthorID)
	if err != nil || author == nil {
		log.Printf("[forum] 取帖子作者失败，跳过回复通知（post_id=%d，author_id=%d）：%v",
			post.ID, post.AuthorID, err)
		return
	}
	// 空邮箱：跳过并记录，不视为失败（AC 明确要求）。
	if author.Detail.Email == "" {
		log.Printf("[forum] 帖子作者没有邮箱，跳过回复通知（post_id=%d，author_id=%d）",
			post.ID, post.AuthorID)
		return
	}

	uc.notify.Enqueue(forumNotifyTask{
		postTitle:   post.Title,
		replyBody:   reply.Body,
		replierName: replierName,
		author:      author,
	})
}
