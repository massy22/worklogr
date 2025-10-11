package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/iriam/worklogr/internal/auth"
	"github.com/iriam/worklogr/internal/config"
	"github.com/iriam/worklogr/internal/utils"
	"golang.org/x/oauth2"
)

// GitHubClient はGitHub API操作を処理します
type GitHubClient struct {
	client          *github.Client
	ctx             context.Context
	user            string
	timezoneManager *utils.TimezoneManager
	authManager     auth.AuthManager
}

// NewGitHubClient は新しいGitHubクライアントを作成します
func NewGitHubClient(token string) (*GitHubClient, error) {
	return NewGitHubClientWithConfig(token, nil)
}

// NewGitHubClientWithConfig は設定付きの新しいGitHubクライアントを作成します
func NewGitHubClientWithConfig(token string, cfg *config.Config) (*GitHubClient, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// 認証されたユーザーを取得
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("認証ユーザーの取得に失敗しました: %w", err)
	}

	// 設定からタイムゾーンマネージャーを作成
	var timezoneManager *utils.TimezoneManager
	var err2 error
	if cfg != nil {
		timezoneManager, err2 = cfg.GetTimezoneManager()
	} else {
		timezoneManager, err2 = utils.NewTimezoneManager("Asia/Tokyo")
	}
	if err2 != nil {
		return nil, fmt.Errorf("タイムゾーンマネージャーの作成に失敗しました: %w", err2)
	}

	return &GitHubClient{
		client:          client,
		ctx:             ctx,
		user:            user.GetLogin(),
		timezoneManager: timezoneManager,
	}, nil
}

// NewGitHubClientWithAuth は認証マネージャー付きの新しいGitHubクライアントを作成します
func NewGitHubClientWithAuth(authManager auth.AuthManager, cfg *config.Config) (*GitHubClient, error) {
	// 認証状態を確認
	ctx := context.Background()
	status, err := authManager.ValidateToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("認証検証に失敗しました: %w", err)
	}

	if !status.IsValid {
		return nil, fmt.Errorf("無効な認証状態: %s", status.ErrorMessage)
	}

	// トークン情報を取得
	tokenInfo := authManager.GetTokenInfo()
	if tokenInfo.TokenType != "access" {
		return nil, fmt.Errorf("無効なトークンタイプ: %s", tokenInfo.TokenType)
	}

	// 認証設定からアクセストークンを取得
	authConfig := authManager.(*auth.GitHubAuthManager).GetConfig()

	// 既存のコンストラクタを使用してクライアントを作成
	githubClient, err := NewGitHubClientWithConfig(authConfig.AccessToken, cfg)
	if err != nil {
		return nil, fmt.Errorf("GitHubクライアントの作成に失敗しました: %w", err)
	}

	// 認証マネージャーを設定
	githubClient.authManager = authManager

	return githubClient, nil
}

// ValidateAuthentication は認証状態を検証します
func (gc *GitHubClient) ValidateAuthentication(ctx context.Context) error {
	if gc.authManager == nil {
		return fmt.Errorf("認証マネージャーが設定されていません")
	}

	status, err := gc.authManager.ValidateToken(ctx)
	if err != nil {
		return fmt.Errorf("認証検証に失敗しました: %w", err)
	}

	if !status.IsValid {
		return fmt.Errorf("認証が無効です: %s", status.ErrorMessage)
	}

	return nil
}

// RefreshAuthenticationIfNeeded は必要に応じて認証をリフレッシュします
func (gc *GitHubClient) RefreshAuthenticationIfNeeded(ctx context.Context) error {
	if gc.authManager == nil {
		return nil // 認証マネージャーがない場合はスキップ
	}

	// GitHubは通常リフレッシュをサポートしないため、検証のみ実行
	return gc.ValidateAuthentication(ctx)
}

// GetAuthStatus は認証状態を取得します
func (gc *GitHubClient) GetAuthStatus() *auth.AuthStatus {
	if gc.authManager == nil {
		return &auth.AuthStatus{
			IsValid:      false,
			LastChecked:  time.Now(),
			ErrorMessage: "認証マネージャーが設定されていません",
			TokenType:    "access",
		}
	}

	return gc.authManager.GetAuthStatus()
}

// CollectGitHubEvents はSearch APIを使用して指定された時間範囲内でGitHubからイベントを収集します
func (gc *GitHubClient) CollectGitHubEvents(startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	fmt.Printf("🔍 %s から %s まで GitHub イベントを収集中\n",
		startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05"))

	// タイムゾーンマネージャーを使用して設定されたタイムゾーンに時刻を変換
	startTimeInTZ := gc.timezoneManager.ConvertToTimezone(startTime)
	endTimeInTZ := gc.timezoneManager.ConvertToTimezone(endTime)

	// GitHub検索用の日付をフォーマット（YYYY-MM-DD形式）
	startDate := startTimeInTZ.Format("2006-01-02")
	endDate := endTimeInTZ.Format("2006-01-02")

	// 検索を使用してコミットを収集
	fmt.Printf("🔍 ユーザー '%s' の GitHub コミットを検索中 (%s から %s)\n", gc.user, startDate, endDate)
	commits, err := gc.searchCommits(startDate, endDate)
	if err != nil {
		fmt.Printf("警告: コミット検索に失敗しました: %v\n", err)
	} else {
		fmt.Printf("   → %d 件のコミットを発見\n", len(commits))
		events = append(events, commits...)
	}

	// 検索を使用してIssueを収集
	fmt.Printf("🔍 '%s' が作成/クローズした GitHub Issue を検索中\n", gc.user)
	issues, err := gc.searchIssues(startDate, endDate)
	if err != nil {
		fmt.Printf("警告: Issue 検索に失敗しました: %v\n", err)
	} else {
		fmt.Printf("   → %d 件の Issue を発見\n", len(issues))
		events = append(events, issues...)
	}

	// 検索を使用してプルリクエストを収集
	fmt.Printf("🔍 '%s' の GitHub プルリクエストを検索中\n", gc.user)
	prs, err := gc.searchPullRequests(startDate, endDate)
	if err != nil {
		fmt.Printf("警告: プルリクエスト検索に失敗しました: %v\n", err)
	} else {
		fmt.Printf("   → %d 件のプルリクエストを発見\n", len(prs))
		events = append(events, prs...)
	}

	// 検索を使用してPRレビューを収集
	fmt.Printf("🔍 '%s' の GitHub PR レビューを検索中\n", gc.user)
	reviews, err := gc.searchPRReviews(startDate, endDate)
	if err != nil {
		fmt.Printf("警告: PR レビュー検索に失敗しました: %v\n", err)
	} else {
		fmt.Printf("   → %d 件のレビューを発見\n", len(reviews))
		events = append(events, reviews...)
	}

	fmt.Printf("✅ 合計 %d 件の GitHub イベントを収集しました\n", len(events))
	return events, nil
}

// getUserRepositories gets repositories for the authenticated user
func (gc *GitHubClient) getUserRepositories() ([]*github.Repository, error) {
	var allRepos []*github.Repository

	opt := &github.RepositoryListOptions{
		Visibility:  "all",
		Affiliation: "owner,collaborator,organization_member",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := gc.client.Repositories.List(gc.ctx, "", opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}

		allRepos = append(allRepos, repos...)

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

// collectCommits collects commits from a repository
func (gc *GitHubClient) collectCommits(repo *github.Repository, startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	opt := &github.CommitsListOptions{
		Author:      gc.user,
		Since:       startTime,
		Until:       endTime,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		commits, resp, err := gc.client.Repositories.ListCommits(gc.ctx, repo.GetOwner().GetLogin(), repo.GetName(), opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list commits: %w", err)
		}

		for _, commit := range commits {
			if commit.GetCommit().GetAuthor().GetDate().Before(startTime) ||
				commit.GetCommit().GetAuthor().GetDate().After(endTime) {
				continue
			}

			event := &config.Event{
				ID:        fmt.Sprintf("github_commit_%s_%s", repo.GetFullName(), commit.GetSHA()),
				Service:   "github",
				Type:      "commit",
				Title:     fmt.Sprintf("Commit to %s", repo.GetFullName()),
				Content:   commit.GetCommit().GetMessage(),
				Timestamp: commit.GetCommit().GetAuthor().GetDate(),
				UserID:    gc.user,
				Metadata:  gc.createCommitMetadata(repo, commit),
			}

			events = append(events, event)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

// collectPullRequests collects pull requests from a repository
func (gc *GitHubClient) collectPullRequests(repo *github.Repository, startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	opt := &github.PullRequestListOptions{
		State:       "all",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		prs, resp, err := gc.client.PullRequests.List(gc.ctx, repo.GetOwner().GetLogin(), repo.GetName(), opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list pull requests: %w", err)
		}

		for _, pr := range prs {
			// Calculate actual end time for filtering
			actualEndTime := endTime
			if endTime.Hour() == 0 && endTime.Minute() == 0 && endTime.Second() == 0 {
				actualEndTime = endTime.Add(24*time.Hour - time.Second)
			}

			// Check if PR was created or updated in the time range
			if pr.GetCreatedAt().After(startTime) && pr.GetCreatedAt().Before(actualEndTime) {
				event := &config.Event{
					ID:        fmt.Sprintf("github_pr_created_%s_%d", repo.GetFullName(), pr.GetNumber()),
					Service:   "github",
					Type:      "pull_request_created",
					Title:     fmt.Sprintf("Created PR #%d in %s", pr.GetNumber(), repo.GetFullName()),
					Content:   pr.GetTitle(),
					Timestamp: pr.GetCreatedAt(),
					UserID:    pr.GetUser().GetLogin(),
					Metadata:  gc.createPRMetadata(repo, pr, "created"),
				}
				events = append(events, event)
			}

			if pr.ClosedAt != nil && pr.ClosedAt.After(startTime) && pr.ClosedAt.Before(actualEndTime) {
				action := "closed"
				if pr.GetMerged() {
					action = "merged"
				}

				event := &config.Event{
					ID:        fmt.Sprintf("github_pr_%s_%s_%d", action, repo.GetFullName(), pr.GetNumber()),
					Service:   "github",
					Type:      fmt.Sprintf("pull_request_%s", action),
					Title:     fmt.Sprintf("%s PR #%d in %s", action, pr.GetNumber(), repo.GetFullName()),
					Content:   pr.GetTitle(),
					Timestamp: *pr.ClosedAt,
					UserID:    pr.GetUser().GetLogin(),
					Metadata:  gc.createPRMetadata(repo, pr, action),
				}
				events = append(events, event)
			}

			// Collect PR review comments
			reviewEvents, err := gc.collectPRReviews(repo, pr, startTime, endTime)
			if err != nil {
				fmt.Printf("Warning: failed to collect PR reviews for #%d: %v\n", pr.GetNumber(), err)
			} else {
				events = append(events, reviewEvents...)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

// collectPRReviews collects pull request reviews
func (gc *GitHubClient) collectPRReviews(repo *github.Repository, pr *github.PullRequest, startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	opt := &github.ListOptions{PerPage: 100}

	for {
		reviews, resp, err := gc.client.PullRequests.ListReviews(gc.ctx, repo.GetOwner().GetLogin(), repo.GetName(), pr.GetNumber(), opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list PR reviews: %w", err)
		}

		for _, review := range reviews {
			if review.SubmittedAt != nil &&
				review.SubmittedAt.After(startTime) &&
				review.SubmittedAt.Before(endTime) &&
				review.GetUser().GetLogin() == gc.user {

				event := &config.Event{
					ID:        fmt.Sprintf("github_pr_review_%s_%d_%d", repo.GetFullName(), pr.GetNumber(), review.GetID()),
					Service:   "github",
					Type:      "pull_request_review",
					Title:     fmt.Sprintf("Reviewed PR #%d in %s", pr.GetNumber(), repo.GetFullName()),
					Content:   review.GetBody(),
					Timestamp: *review.SubmittedAt,
					UserID:    review.GetUser().GetLogin(),
					Metadata:  gc.createPRReviewMetadata(repo, pr, review),
				}
				events = append(events, event)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

// collectIssues collects issues from a repository
func (gc *GitHubClient) collectIssues(repo *github.Repository, startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	opt := &github.IssueListByRepoOptions{
		State:       "all",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		issues, resp, err := gc.client.Issues.ListByRepo(gc.ctx, repo.GetOwner().GetLogin(), repo.GetName(), opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list issues: %w", err)
		}

		for _, issue := range issues {
			// Skip pull requests (they appear in issues API)
			if issue.IsPullRequest() {
				continue
			}

			// Calculate actual end time for filtering
			actualEndTime := endTime
			if endTime.Hour() == 0 && endTime.Minute() == 0 && endTime.Second() == 0 {
				actualEndTime = endTime.Add(24*time.Hour - time.Second)
			}

			// Check if issue was created in the time range
			if issue.GetCreatedAt().After(startTime) && issue.GetCreatedAt().Before(actualEndTime) {
				event := &config.Event{
					ID:        fmt.Sprintf("github_issue_created_%s_%d", repo.GetFullName(), issue.GetNumber()),
					Service:   "github",
					Type:      "issue_created",
					Title:     fmt.Sprintf("Created issue #%d in %s", issue.GetNumber(), repo.GetFullName()),
					Content:   issue.GetTitle(),
					Timestamp: issue.GetCreatedAt(),
					UserID:    issue.GetUser().GetLogin(),
					Metadata:  gc.createIssueMetadata(repo, issue, "created"),
				}
				events = append(events, event)
			}

			// Check if issue was closed in the time range
			if issue.ClosedAt != nil && issue.ClosedAt.After(startTime) && issue.ClosedAt.Before(actualEndTime) {
				event := &config.Event{
					ID:        fmt.Sprintf("github_issue_closed_%s_%d", repo.GetFullName(), issue.GetNumber()),
					Service:   "github",
					Type:      "issue_closed",
					Title:     fmt.Sprintf("Closed issue #%d in %s", issue.GetNumber(), repo.GetFullName()),
					Content:   issue.GetTitle(),
					Timestamp: *issue.ClosedAt,
					UserID:    issue.GetUser().GetLogin(),
					Metadata:  gc.createIssueMetadata(repo, issue, "closed"),
				}
				events = append(events, event)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

// collectReleases collects releases from a repository
func (gc *GitHubClient) collectReleases(repo *github.Repository, startTime, endTime time.Time) ([]*config.Event, error) {
	var events []*config.Event

	opt := &github.ListOptions{PerPage: 100}

	for {
		releases, resp, err := gc.client.Repositories.ListReleases(gc.ctx, repo.GetOwner().GetLogin(), repo.GetName(), opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list releases: %w", err)
		}

		for _, release := range releases {
			// Calculate actual end time for filtering
			actualEndTime := endTime
			if endTime.Hour() == 0 && endTime.Minute() == 0 && endTime.Second() == 0 {
				actualEndTime = endTime.Add(24*time.Hour - time.Second)
			}

			createdAt := release.GetCreatedAt().Time
			if createdAt.After(startTime) && createdAt.Before(actualEndTime) {
				userID := gc.user
				if release.GetAuthor() != nil {
					userID = release.GetAuthor().GetLogin()
				}

				event := &config.Event{
					ID:        fmt.Sprintf("github_release_%s_%d", repo.GetFullName(), release.GetID()),
					Service:   "github",
					Type:      "release_created",
					Title:     fmt.Sprintf("Created release %s in %s", release.GetTagName(), repo.GetFullName()),
					Content:   release.GetName(),
					Timestamp: createdAt,
					UserID:    userID,
					Metadata:  gc.createReleaseMetadata(repo, release),
				}
				events = append(events, event)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

// createCommitMetadata creates metadata for a commit event
func (gc *GitHubClient) createCommitMetadata(repo *github.Repository, commit *github.RepositoryCommit) string {
	metadata := map[string]interface{}{
		"repository":    repo.GetFullName(),
		"sha":           commit.GetSHA(),
		"url":           commit.GetHTMLURL(),
		"additions":     commit.GetStats().GetAdditions(),
		"deletions":     commit.GetStats().GetDeletions(),
		"changed_files": len(commit.Files),
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}

// createPRMetadata creates metadata for a pull request event
func (gc *GitHubClient) createPRMetadata(repo *github.Repository, pr *github.PullRequest, action string) string {
	metadata := map[string]interface{}{
		"repository": repo.GetFullName(),
		"number":     pr.GetNumber(),
		"action":     action,
		"url":        pr.GetHTMLURL(),
		"state":      pr.GetState(),
		"base":       pr.GetBase().GetRef(),
		"head":       pr.GetHead().GetRef(),
		"additions":  pr.GetAdditions(),
		"deletions":  pr.GetDeletions(),
		"commits":    pr.GetCommits(),
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}

// createPRReviewMetadata creates metadata for a pull request review event
func (gc *GitHubClient) createPRReviewMetadata(repo *github.Repository, pr *github.PullRequest, review *github.PullRequestReview) string {
	metadata := map[string]interface{}{
		"repository":   repo.GetFullName(),
		"pr_number":    pr.GetNumber(),
		"review_id":    review.GetID(),
		"review_state": review.GetState(),
		"url":          review.GetHTMLURL(),
		"pr_url":       pr.GetHTMLURL(),
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}

// createIssueMetadata creates metadata for an issue event
func (gc *GitHubClient) createIssueMetadata(repo *github.Repository, issue *github.Issue, action string) string {
	metadata := map[string]interface{}{
		"repository": repo.GetFullName(),
		"number":     issue.GetNumber(),
		"action":     action,
		"url":        issue.GetHTMLURL(),
		"state":      issue.GetState(),
		"labels":     issue.Labels,
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}

// createReleaseMetadata creates metadata for a release event
func (gc *GitHubClient) createReleaseMetadata(repo *github.Repository, release *github.RepositoryRelease) string {
	metadata := map[string]interface{}{
		"repository": repo.GetFullName(),
		"tag_name":   release.GetTagName(),
		"name":       release.GetName(),
		"url":        release.GetHTMLURL(),
		"prerelease": release.GetPrerelease(),
		"draft":      release.GetDraft(),
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}

// Search API functions for efficient data collection

// searchCommits searches for commits using GitHub Search API
func (gc *GitHubClient) searchCommits(startDate, endDate string) ([]*config.Event, error) {
	var events []*config.Event

	query := fmt.Sprintf("author:%s created:%s..%s", gc.user, startDate, endDate)

	opt := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := gc.client.Search.Commits(gc.ctx, query, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to search commits: %w", err)
		}

		for _, commit := range result.Commits {
			event := &config.Event{
				ID:        fmt.Sprintf("github_commit_%s_%s", commit.Repository.GetFullName(), commit.GetSHA()),
				Service:   "github",
				Type:      "commit",
				Title:     fmt.Sprintf("Commit to %s", commit.Repository.GetFullName()),
				Content:   commit.Commit.GetMessage(),
				Timestamp: commit.Commit.Author.GetDate(),
				UserID:    gc.user,
				Metadata:  gc.createSimpleCommitMetadata(commit),
			}
			events = append(events, event)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

// searchIssues searches for issues using GitHub Search API
func (gc *GitHubClient) searchIssues(startDate, endDate string) ([]*config.Event, error) {
	var events []*config.Event

	// Search for issues created by user
	createdQuery := fmt.Sprintf("author:%s type:issue created:%s..%s", gc.user, startDate, endDate)
	createdEvents, err := gc.searchIssuesByQuery(createdQuery, "issue_created")
	if err != nil {
		return nil, fmt.Errorf("failed to search created issues: %w", err)
	}
	events = append(events, createdEvents...)

	// Search for issues closed by user
	closedQuery := fmt.Sprintf("assignee:%s type:issue closed:%s..%s", gc.user, startDate, endDate)
	closedEvents, err := gc.searchIssuesByQuery(closedQuery, "issue_closed")
	if err != nil {
		return nil, fmt.Errorf("failed to search closed issues: %w", err)
	}
	events = append(events, closedEvents...)

	return events, nil
}

// searchPullRequests searches for pull requests using GitHub Search API
func (gc *GitHubClient) searchPullRequests(startDate, endDate string) ([]*config.Event, error) {
	var events []*config.Event

	// Search for PRs created by user
	createdQuery := fmt.Sprintf("author:%s type:pr created:%s..%s", gc.user, startDate, endDate)
	createdEvents, err := gc.searchPRsByQuery(createdQuery, "pull_request_created")
	if err != nil {
		return nil, fmt.Errorf("failed to search created PRs: %w", err)
	}
	events = append(events, createdEvents...)

	// Search for PRs merged by user
	mergedQuery := fmt.Sprintf("author:%s type:pr merged:%s..%s", gc.user, startDate, endDate)
	mergedEvents, err := gc.searchPRsByQuery(mergedQuery, "pull_request_merged")
	if err != nil {
		return nil, fmt.Errorf("failed to search merged PRs: %w", err)
	}
	events = append(events, mergedEvents...)

	// Search for PRs closed by user
	closedQuery := fmt.Sprintf("author:%s type:pr closed:%s..%s", gc.user, startDate, endDate)
	closedEvents, err := gc.searchPRsByQuery(closedQuery, "pull_request_closed")
	if err != nil {
		return nil, fmt.Errorf("failed to search closed PRs: %w", err)
	}
	events = append(events, closedEvents...)

	return events, nil
}

// searchPRReviews searches for PR reviews using GitHub Search API
func (gc *GitHubClient) searchPRReviews(startDate, endDate string) ([]*config.Event, error) {
	var events []*config.Event

	// Search for PRs reviewed by user
	query := fmt.Sprintf("reviewed-by:%s type:pr updated:%s..%s", gc.user, startDate, endDate)

	opt := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	// Track processed PRs to avoid duplicate API calls
	processedPRs := make(map[string]bool)
	totalPRs := 0
	processedCount := 0

	for {
		result, resp, err := gc.client.Search.Issues(gc.ctx, query, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to search PR reviews: %w", err)
		}

		totalPRs += len(result.Issues)

		for _, issue := range result.Issues {
			// Convert issue to PR and get reviews
			if issue.IsPullRequest() {
				// Extract repository info from URL
				repoInfo := gc.extractRepoFromURL(issue.GetRepositoryURL())
				if repoInfo.Owner == "" || repoInfo.Name == "" {
					continue
				}

				// Create unique key for this PR
				prKey := fmt.Sprintf("%s/%s#%d", repoInfo.Owner, repoInfo.Name, issue.GetNumber())

				// Skip if already processed
				if processedPRs[prKey] {
					continue
				}
				processedPRs[prKey] = true
				processedCount++

				// Show progress for large datasets
				if processedCount%10 == 0 {
					fmt.Printf("   レビュー取得中... %d/%d PRs 処理済み\n", processedCount, totalPRs)
				}

				// Get PR reviews to find the specific review by this user
				reviews, err := gc.getPRReviewsInDateRange(repoInfo.Owner, repoInfo.Name, issue.GetNumber(), startDate, endDate)
				if err != nil {
					fmt.Printf("警告: PR #%d のレビュー取得に失敗しました: %v\n", issue.GetNumber(), err)
					continue
				}
				events = append(events, reviews...)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

// Helper functions for search

func (gc *GitHubClient) searchIssuesByQuery(query, eventType string) ([]*config.Event, error) {
	var events []*config.Event

	opt := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := gc.client.Search.Issues(gc.ctx, query, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to search issues: %w", err)
		}

		for _, issue := range result.Issues {
			if !issue.IsPullRequest() {
				var timestamp time.Time
				if eventType == "issue_created" {
					timestamp = issue.GetCreatedAt()
				} else if eventType == "issue_closed" && issue.ClosedAt != nil {
					timestamp = *issue.ClosedAt
				} else {
					continue
				}

				repoInfo := gc.extractRepoFromURL(issue.GetRepositoryURL())
				event := &config.Event{
					ID:        fmt.Sprintf("github_%s_%s_%d", eventType, repoInfo.FullName, issue.GetNumber()),
					Service:   "github",
					Type:      eventType,
					Title:     fmt.Sprintf("%s #%d in %s", gc.getActionTitle(eventType), issue.GetNumber(), repoInfo.FullName),
					Content:   issue.GetTitle(),
					Timestamp: timestamp,
					UserID:    gc.user,
					Metadata:  gc.createSimpleIssueMetadata(issue, repoInfo.FullName, eventType),
				}
				events = append(events, event)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

func (gc *GitHubClient) searchPRsByQuery(query, eventType string) ([]*config.Event, error) {
	var events []*config.Event

	opt := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := gc.client.Search.Issues(gc.ctx, query, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to search PRs: %w", err)
		}

		for _, issue := range result.Issues {
			if issue.IsPullRequest() {
				var timestamp time.Time
				if eventType == "pull_request_created" {
					timestamp = issue.GetCreatedAt()
				} else if eventType == "pull_request_merged" && issue.ClosedAt != nil {
					timestamp = *issue.ClosedAt
				} else if eventType == "pull_request_closed" && issue.ClosedAt != nil {
					timestamp = *issue.ClosedAt
				} else {
					continue
				}

				repoInfo := gc.extractRepoFromURL(issue.GetRepositoryURL())
				event := &config.Event{
					ID:        fmt.Sprintf("github_%s_%s_%d", eventType, repoInfo.FullName, issue.GetNumber()),
					Service:   "github",
					Type:      eventType,
					Title:     fmt.Sprintf("%s #%d in %s", gc.getActionTitle(eventType), issue.GetNumber(), repoInfo.FullName),
					Content:   issue.GetTitle(),
					Timestamp: timestamp,
					UserID:    gc.user,
					Metadata:  gc.createSimplePRMetadata(issue, repoInfo.FullName, eventType),
				}
				events = append(events, event)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

type RepoInfo struct {
	Owner    string
	Name     string
	FullName string
}

func (gc *GitHubClient) extractRepoFromURL(url string) RepoInfo {
	// Extract owner/repo from repository URL like "https://api.github.com/repos/owner/repo"
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		owner := parts[len(parts)-2]
		name := parts[len(parts)-1]
		return RepoInfo{
			Owner:    owner,
			Name:     name,
			FullName: fmt.Sprintf("%s/%s", owner, name),
		}
	}
	return RepoInfo{}
}

func (gc *GitHubClient) getPRReviewsInDateRange(owner, repo string, prNumber int, startDate, endDate string) ([]*config.Event, error) {
	var events []*config.Event

	opt := &github.ListOptions{PerPage: 100}

	for {
		reviews, resp, err := gc.client.PullRequests.ListReviews(gc.ctx, owner, repo, prNumber, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list PR reviews: %w", err)
		}

		for _, review := range reviews {
			if review.SubmittedAt != nil && review.GetUser().GetLogin() == gc.user {
				// Check if review is in date range
				reviewDate := review.SubmittedAt.Format("2006-01-02")
				if reviewDate >= startDate && reviewDate <= endDate {
					event := &config.Event{
						ID:        fmt.Sprintf("github_pr_review_%s_%d_%d", fmt.Sprintf("%s/%s", owner, repo), prNumber, review.GetID()),
						Service:   "github",
						Type:      "pull_request_review",
						Title:     fmt.Sprintf("Reviewed PR #%d in %s/%s", prNumber, owner, repo),
						Content:   review.GetBody(),
						Timestamp: *review.SubmittedAt,
						UserID:    gc.user,
						Metadata:  gc.createSimpleReviewMetadata(review, fmt.Sprintf("%s/%s", owner, repo), prNumber),
					}
					events = append(events, event)
				}
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return events, nil
}

func (gc *GitHubClient) getActionTitle(eventType string) string {
	switch eventType {
	case "issue_created":
		return "Created issue"
	case "issue_closed":
		return "Closed issue"
	case "pull_request_created":
		return "Created PR"
	case "pull_request_merged":
		return "Merged PR"
	case "pull_request_closed":
		return "Closed PR"
	default:
		return "Action"
	}
}

// Simplified metadata functions (without detailed stats)

func (gc *GitHubClient) createSimpleCommitMetadata(commit *github.CommitResult) string {
	metadata := map[string]interface{}{
		"repository": commit.Repository.GetFullName(),
		"sha":        commit.GetSHA(),
		"url":        commit.GetHTMLURL(),
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}

func (gc *GitHubClient) createSimpleIssueMetadata(issue *github.Issue, repoName, action string) string {
	metadata := map[string]interface{}{
		"repository": repoName,
		"number":     issue.GetNumber(),
		"action":     action,
		"url":        issue.GetHTMLURL(),
		"state":      issue.GetState(),
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}

func (gc *GitHubClient) createSimplePRMetadata(issue *github.Issue, repoName, action string) string {
	metadata := map[string]interface{}{
		"repository": repoName,
		"number":     issue.GetNumber(),
		"action":     action,
		"url":        issue.GetHTMLURL(),
		"state":      issue.GetState(),
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}

func (gc *GitHubClient) createSimpleReviewMetadata(review *github.PullRequestReview, repoName string, prNumber int) string {
	metadata := map[string]interface{}{
		"repository":   repoName,
		"pr_number":    prNumber,
		"review_id":    review.GetID(),
		"review_state": review.GetState(),
		"url":          review.GetHTMLURL(),
	}

	data, _ := json.Marshal(metadata)
	return string(data)
}
