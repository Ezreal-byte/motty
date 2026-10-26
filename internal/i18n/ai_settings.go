package i18n

// AISettingsMessages are the texts of the assistant's settings section.
type AISettingsMessages struct {
	Title       string
	Description string
	// StoreErrorFmt takes the reason the assistant's database could not be used.
	StoreErrorFmt string

	// Providers.
	ProvidersTitle     string
	ProvidersEmpty     string
	ProviderAdd        string
	ProviderEdit       string
	ProviderDelete     string
	ProviderEnabled    string // accessible name of a provider's switch
	ProviderKeySaved   string
	ProviderNoKey      string
	ProviderOfficial   string // shown for a provider that uses its official endpoint
	ProviderKindOpenAI string
	ProviderKindClaude string

	// The provider dialog.
	FormAddTitle       string
	FormEditTitle      string
	FormDescription    string
	FormName           string
	FormNamePH         string
	FormKind           string
	FormPreset         string
	FormPresetCustom   string
	FormBaseURL        string
	FormBaseURLOpenAI  string // placeholder: the official default
	FormBaseURLClaude  string
	FormKey            string
	FormKeyPH          string
	FormKeySaved       string
	FormSave           string
	FormAdd            string
	ErrFormName        string
	ErrFormBaseURL     string
	ErrFormKey         string
	ErrFormSaveFmt     string
	ErrModelName       string
	CLIPath            string
	CLIPathDefault     string
	CLIFormDescription string
	CLIDefaultModel    string
	CLIModelsNote      string
	CLIModelsCheck     string

	CLIAuthTitle      string
	CLIAuthLogin      string
	CLIAuthDevice     string
	CLIAuthRecheck    string
	CLIAuthUnknown    string
	CLIAuthSignedOut  string
	CLIAuthConfigured string
	CLIAuthChecking   string
	CLIAuthSigningIn  string
	CLIAuthFailed     string
	CLIAuthCanceled   string
	CLIAuthTimeout    string
	CLIAuthSensitive  string
	CLIErrInstall     string
	CLIErrVersion     string
	CLIErrLogin       string
	CLIErrRunFmt      string

	// Deleting a provider.
	DeleteTitle   string
	DeleteBodyFmt string
	DeleteConfirm string

	// Models.
	ModelsTitle      string
	ModelsNoProvider string
	ModelsOfFmt      string
	ModelsDescFmt    string
	ModelsFetch      string
	ModelsFetching   string
	ModelsCancel     string
	ModelsFetchedFmt string
	ModelsFailedFmt  string
	ModelsEmpty      string
	ModelDefault     string
	ModelUse         string
	ModelEnabled     string // accessible name of a model's switch
	ModelAddPH       string
	ModelAdd         string
	ModelPageFmt     string
	ModelPrev        string
	ModelNext        string

	// Behaviour.
	BehaviourTitle   string
	AutoRun          string
	AutoRunDesc      string
	RiskLow          string
	RiskMedium       string
	RiskHigh         string
	MaxSteps         string
	MaxStepsDesc     string
	ContextLines     string
	ContextLinesDesc string
	ContextOff       string
	Redact           string
	RedactDesc       string
	History          string
	HistoryDesc      string
	ClearHistory     string
	ClearHistoryDesc string
	ClearHistoryDone string
	ClearHistoryBusy string
	ClearTitle       string
	ClearBody        string
	ClearConfirm     string

	// External access.
	MCPEnable        string
	MCPEnableDesc    string
	MCPMode          string
	MCPModeDesc      string
	MCPModeObserve   string
	MCPModeAsk       string
	MCPEndpoint      string
	MCPNotRunning    string
	MCPOff           string
	MCPCopy          string
	MCPCopied        string
	MCPCopyFailed    string
	MCPToken         string
	MCPTokenDesc     string
	MCPReset         string
	MCPResetDone     string
	MCPConfig        string
	MCPConfigDesc    string
	MCPCopyURL       string
	MCPCopyToken     string
	MCPClaudeHint    string
	MCPCursorHint    string
	MCPCodexHint     string
	MCPCherryHint    string
	MCPCodeBuddyHint string
	MCPDeepSeekHint  string
	MCPOpenCodeHint  string
	MCPQoderHint     string
	MCPTraeHint      string
	MCPVSCodeHint    string
	MCPWindsurfHint  string
	MCPWorkBuddyHint string
	MCPZCodeHint     string
	MCPClientLabel   string
	MCPPreviewHint   string
}
