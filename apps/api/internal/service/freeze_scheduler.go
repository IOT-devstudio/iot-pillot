package service

import (
	"context"
	"log"
	"sync"
	"time"
)

// FreezeScheduler 把定时冻结任务挂在进程生命周期上（0012）。
//
// 为什么是进程内 ticker 而不是外部 cron：
//   - 部署是单镜像 + 单容器，没有可依赖的宿主 cron；
//   - 冻结不是实时风控，晚几小时没有实际损失，进程内最省运维；
//   - 幂等由仓储层的条件更新（frozen_at IS NULL）保证，多副本重复执行
//     也不会重复冻结或重复发信。
//
// 可测性：Start/Stop 之外不藏全局状态，RunOnce 本身是纯编排，
// 因此单测直接调用 FreezeUseCase.RunOnce 即可，不必启动 ticker。
type FreezeScheduler struct {
	useCase  *FreezeUseCase
	interval time.Duration

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// NewFreezeScheduler 构造调度器（不启动）。
func NewFreezeScheduler(useCase *FreezeUseCase) *FreezeScheduler {
	return &FreezeScheduler{
		useCase:  useCase,
		interval: FreezeScanInterval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start 启动后台循环：立即扫一次，之后每 interval 扫一次，直到 Stop。
//
// 启动即扫的理由：容器重启频繁，若只等满一个周期才执行，滚动发布期间
// 「该冻结的账号」会一直被推迟；而扫描本身幂等，多扫一次没有副作用。
func (s *FreezeScheduler) Start() {
	go func() {
		defer close(s.done)

		s.runOnce("启动")

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runOnce("定时")
			case <-s.stop:
				return
			}
		}
	}()
}

// Stop 停止后台循环并等待当前一轮结束。
//
// 等待是必要的：进程退出时若正处在「已发通知、未写冻结」之间，
// 下一轮会重发通知。等这一轮落库完成，状态才是自洽的。
// 重复调用安全（sync.Once）。
func (s *FreezeScheduler) Stop() {
	s.stopOnce.Do(func() {
		close(s.stop)
		<-s.done
	})
}

// runOnce 执行一轮并只记日志，不向上抛错。
//
// 后台任务没有调用方可以接收错误：错误只可能来自数据库不可用等基础设施
// 问题，记日志后等下一轮即可；抛出去也没有人能处理。
func (s *FreezeScheduler) runOnce(trigger string) {
	// 用独立 context：调度器的生命周期由 Stop 控制，不跟随单个请求。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := s.useCase.RunOnce(ctx)
	if err != nil {
		log.Printf("[freeze] %s扫描失败（下一轮重试）: %v", trigger, err)
		return
	}

	log.Printf("[freeze] %s扫描完成：候选 %d，合规 %d，冻结 %d，已被冻结 %d，邮件失败 %d，跳过管理员 %d，解冻管理员 %d",
		trigger, result.Candidates, result.Compliant, result.Frozen, result.AlreadyFrozen,
		result.MailFailed, result.SkippedAdmins, result.RescuedAdmins)
}
