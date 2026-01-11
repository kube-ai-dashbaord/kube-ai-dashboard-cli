package i18n

import (
	"strings"
)

type Language string

const (
	EN Language = "en"
	KO Language = "ko"
	ZH Language = "zh"
	JA Language = "ja"
)

var currentLang Language = EN

func SetLanguage(lang string) {
	switch strings.ToLower(lang) {
	case "ko", "korean":
		currentLang = KO
	case "zh", "chinese":
		currentLang = ZH
	case "ja", "japanese":
		currentLang = JA
	default:
		currentLang = EN
	}
}

func GetLanguage() Language {
	return currentLang
}

var translations = map[Language]map[string]string{
	EN: {
		"app_title":         "k13s - K8s AI Explorer",
		"dashboard_pods":    "Dashboard (Pods)",
		"ask_ai":            "Ask AI: ",
		"decision_required": "Decision Required",
		"settings_title":    "LLM Settings",
		"audit_logs":        "Audit Logs",
		"help_title":        "k13s Help & Shortcuts",
		"shortcut_help":     "?:Help",
		"shortcut_cmd":      "/:Command",
		"shortcut_settings": "s:Settings",
		"shortcut_yaml":     "y:YAML",
		"shortcut_describe": "d:Describe",
		"shortcut_analyze":  "L:AI Analyze",
		"shortcut_forward":  "Shift-F:Forward",
		"shortcut_quit":     "Ctrl-C:Quit",
		"explain_this":      "Explain this resource",
		"beginner_mode":     "Beginner Mode",
		"loading":           "Loading...",
		"error_client":      "K8s Client not initialized",
		"cat_nav":           "General Navigation",
		"cat_dash":          "Dashboard Actions",
		"cat_res":           "Resource Actions",
		"desc_switch":       "Switch resource (e.g., :pods, :svc)",
		"desc_ctx":          "Switch context",
		"desc_audit":        "View enterprise audit logs",
		"desc_filter":       "Filter table rows",
		"desc_yaml":         "View YAML",
		"desc_describe":     "Native resource description",
		"desc_analyze":      "AI-powered analysis",
		"desc_explain":      "AI-powered explanation",
		"desc_scale":        "Scale resource",
		"desc_restart":      "Rollout restart",
		"desc_forward":      "Port Forward",
		"desc_logs":         "Stream logs",
		"desc_delete":       "Delete resource",
	},
	KO: {
		"app_title":         "k13s - K8s AI 탐색기",
		"dashboard_pods":    "대시보드 (Pods)",
		"ask_ai":            "AI에게 질문: ",
		"decision_required": "의사결정 필요",
		"settings_title":    "LLM 설정",
		"audit_logs":        "감사 로그",
		"help_title":        "k13s 도움말 및 단축키",
		"shortcut_help":     "?:도움말",
		"shortcut_cmd":      "/:명령어",
		"shortcut_settings": "s:설정",
		"shortcut_yaml":     "y:YAML",
		"shortcut_describe": "d:설명",
		"shortcut_analyze":  "L:AI 분석",
		"shortcut_forward":  "Shift-F:포워딩",
		"shortcut_quit":     "Ctrl-C:종료",
		"explain_this":      "이 리소스 설명하기",
		"beginner_mode":     "초보자 모드",
		"loading":           "로딩 중...",
		"error_client":      "K8s 클라이언트가 초기화되지 않았습니다",
		"cat_nav":           "일반 탐색",
		"cat_dash":          "대시보드 작업",
		"cat_res":           "리소스 작업",
		"desc_switch":       "리소스 전환 (예: :pods, :svc)",
		"desc_ctx":          "컨텍스트 전환",
		"desc_audit":        "엔터프라이즈 감사 로그 보기",
		"desc_filter":       "테이블 필터링",
		"desc_yaml":         "YAML 보기",
		"desc_describe":     "네이티브 리소스 설명 보기",
		"desc_analyze":      "AI 기반 분석",
		"desc_explain":      "AI 기반 설명",
		"desc_scale":        "리소스 스케일링",
		"desc_restart":      "재시작 (Rollout)",
		"desc_forward":      "포트 포워딩",
		"desc_logs":         "로그 스트리밍",
		"desc_delete":       "리소스 삭제",
	},
	ZH: {
		"app_title":         "k13s - K8s AI 资源管理器",
		"dashboard_pods":    "仪表板 (Pods)",
		"ask_ai":            "向 AI 提问: ",
		"decision_required": "需要决策",
		"settings_title":    "LLM 设置",
		"audit_logs":        "审计日志",
		"help_title":        "k13s 帮助与快捷键",
		"shortcut_help":     "?:帮助",
		"shortcut_cmd":      "/:命令",
		"shortcut_settings": "s:设置",
		"shortcut_yaml":     "y:YAML",
		"shortcut_describe": "d:详情",
		"shortcut_analyze":  "L:AI 分析",
		"shortcut_forward":  "Shift-F:转发",
		"shortcut_quit":     "Ctrl-C:退出",
		"explain_this":      "解释此资源",
		"beginner_mode":     "入门模式",
		"loading":           "加载中...",
		"error_client":      "K8s 客户端未初始化",
		"cat_nav":           "通用导航",
		"cat_dash":          "仪表板操作",
		"cat_res":           "资源操作",
		"desc_switch":       "切换资源 (如 :pods, :svc)",
		"desc_ctx":          "切换上下文",
		"desc_audit":        "查看审计日志",
		"desc_filter":       "过滤表格行",
		"desc_yaml":         "查看 YAML",
		"desc_describe":     "原生资源描述",
		"desc_analyze":      "AI 驱动分析",
		"desc_explain":      "AI 驱动解释",
		"desc_scale":        "调整副本数",
		"desc_restart":      "重启资源",
		"desc_forward":      "端口转发",
		"desc_logs":         "实时日志",
		"desc_delete":       "删除资源",
	},
	JA: {
		"app_title":         "k13s - K8s AI エクスプローラー",
		"dashboard_pods":    "ダッシュボード (Pods)",
		"ask_ai":            "AIに質問: ",
		"decision_required": "決定が必要",
		"settings_title":    "LLM 設定",
		"audit_logs":        "監査ログ",
		"help_title":        "k13s ヘルプとショートカット",
		"shortcut_help":     "?:ヘルプ",
		"shortcut_cmd":      "/:コマンド",
		"shortcut_settings": "s:設定",
		"shortcut_yaml":     "y:YAML",
		"shortcut_describe": "d:詳細",
		"shortcut_analyze":  "L:AI 分析",
		"shortcut_forward":  "Shift-F:フォワード",
		"shortcut_quit":     "Ctrl-C:終了",
		"explain_this":      "このリソースを解説",
		"beginner_mode":     "初心者モード",
		"loading":           "読み込み中...",
		"error_client":      "K8s クライアントが初期化されていません",
		"cat_nav":           "基本ナビゲーション",
		"cat_dash":          "ダッシュボード操作",
		"cat_res":           "リソース操作",
		"desc_switch":       "リソース切り替え (:pods, :svc)",
		"desc_ctx":          "コンテキスト切り替え",
		"desc_audit":        "監査ログを表示",
		"desc_filter":       "テーブルをフィルタ",
		"desc_yaml":         "YAMLを表示",
		"desc_describe":     "ネイティブの詳細表示",
		"desc_analyze":      "AIによる分析",
		"desc_explain":      "AIによる解説",
		"desc_scale":        "スケール調整",
		"desc_restart":      "ロールアウト再起動",
		"desc_forward":      "ポートフォワード",
		"desc_logs":         "ログのストリーム",
		"desc_delete":       "リソース削除",
	},
}

func T(key string) string {
	if langMap, ok := translations[currentLang]; ok {
		if val, ok := langMap[key]; ok {
			return val
		}
	}
	// Fallback to English
	if langMap, ok := translations[EN]; ok {
		if val, ok := langMap[key]; ok {
			return val
		}
	}
	return key
}
