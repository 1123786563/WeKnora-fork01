package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// forkLineageBody 是端到端断言用的 submission wire 形状。manifest 是内嵌
// JSON 对象，解码为 RawMessage 后做字符串断言。
type forkLineageBody struct {
	Data struct {
		ID                  string          `json:"id"`
		ListingID           string          `json:"listing_id"`
		BundleDigest        string          `json:"bundle_digest"`
		IsFork              bool            `json:"is_fork"`
		ForkSourceListingID string          `json:"fork_source_listing_id"`
		ForkSourceReleaseID string          `json:"fork_source_release_id"`
		ForkNotes           string          `json:"fork_notes"`
		LineageLicenseID    string          `json:"lineage_license_id"`
		Manifest            json.RawMessage `json:"manifest"`
	} `json:"data"`
}

// forkReviewBody 是 review 端点的 release 解码形状。
type forkReviewBody struct {
	Data struct {
		Release *struct {
			ID string `json:"id"`
		} `json:"release"`
	} `json:"data"`
}

// registerForkLicense 注册（或翻转）一条部署许可证（Admin+ 端点）。
func registerForkLicense(t *testing.T, r *gin.Engine, id string, allowsRedistribution bool) {
	t.Helper()
	registered := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/licenses", "admin", "admin", map[string]any{
		"id": id, "name": "License " + id, "allows_redistribution": allowsRedistribution,
	})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, registered.Code, registered.Body.String())
}

// seedForkChain 走真实 HTTP adoption 链：adopt → variant → 完整映射（含一次
// 重映射，即 AC1 的「Mapping 修改」）→ test → publish →（editedPrompt 非空时
// 修改可移植核心并 freeze 新版本）。返回 (localAgentID, 可提交的 versionID)：
// editedPrompt 为空时是发布时冻结的版本（纯映射派生），非空时是 fork 冻结版本。
func seedForkChain(t *testing.T, r *gin.Engine, listingID, editedPrompt string) (string, string) {
	t.Helper()
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		return publicCall(r, 1, false, method, path, "admin", "admin", body)
	}

	adopted := call(http.MethodPost, "/api/v1/marketplace/tenant/adoptions", map[string]any{"listing_id": listingID})
	// 首次 201（created）；同一 listing 再派生时是幂等 re-adopt → 200，
	// accepted 指针指向 listing 当前 Release（201/200 都合法）。
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, adopted.Code, adopted.Body.String())
	var adoption struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(adopted.Body.Bytes(), &adoption))

	variant := call(http.MethodPost, "/api/v1/marketplace/tenant/adoptions/"+adoption.Data.ID+"/variants", map[string]any{"name": "Sales Assistant"})
	require.Equal(t, http.StatusCreated, variant.Code, variant.Body.String())
	var variantBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(variant.Body.Bytes(), &variantBody))

	// 完整映射后再重映射一次（换知识库绑定）：AC1 的「Mapping 修改」。
	for _, kb := range []string{"kb-sales", "kb-legal"} {
		mapped := call(http.MethodPut, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/capability-mapping", map[string]any{"mappings": []map[string]any{
			{"capability": "model", "model_id": "gpt-x"},
			{"capability": "knowledge", "knowledge_base_ids": []string{kb}},
		}})
		require.Equal(t, http.StatusOK, mapped.Code, mapped.Body.String())
	}

	tested := call(http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/test", nil)
	require.Equal(t, http.StatusOK, tested.Code, tested.Body.String())
	published := call(http.MethodPost, "/api/v1/marketplace/tenant/variants/"+variantBody.Data.ID+"/publish", nil)
	require.Equal(t, http.StatusOK, published.Code, published.Body.String())
	var publishBody struct {
		Data struct {
			LocalAgentID        string `json:"local_agent_id"`
			LocalAgentVersionID string `json:"local_agent_version_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(published.Body.Bytes(), &publishBody))
	require.NotEmpty(t, publishBody.Data.LocalAgentID)
	require.NotEmpty(t, publishBody.Data.LocalAgentVersionID)

	if editedPrompt == "" {
		return publishBody.Data.LocalAgentID, publishBody.Data.LocalAgentVersionID
	}
	edited := call(http.MethodPut, "/api/v1/agents/"+publishBody.Data.LocalAgentID, map[string]any{
		"name": "Sales Assistant", "description": "forked",
		"config": map[string]any{"agent_mode": "smart-reasoning", "system_prompt": editedPrompt},
	})
	require.Equal(t, http.StatusOK, edited.Code, edited.Body.String())
	frozen := call(http.MethodPost, "/api/v1/agents/"+publishBody.Data.LocalAgentID+"/versions", nil)
	require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
	var frozenBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &frozenBody))
	return publishBody.Data.LocalAgentID, frozenBody.Data.ID
}

// submitForkVersionRaw 提交一个冻结版本，返回原始响应（供 409 断言）。
// metadata 统一带 capability_requirements：源 listing 上的 release 必须声明
// model/knowledge 需求，adoption 派生链的 capability-mapping 才能通过
// （UpdateCapabilityMapping 拒绝 release 未声明的 capability）；fork 提交
// 多带该字段无害。
func submitForkVersionRaw(r *gin.Engine, role, actor, versionID, licenseID, version, notes string) *httptest.ResponseRecorder {
	return publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", role, actor, map[string]any{
		"agent_version_id": versionID,
		"metadata": map[string]any{
			"semantic_version": version, "display_name": "Derived helper", "summary": "Derived",
			"supported_languages": []string{"en"}, "use_cases": []string{"support"},
			"capability_requirements":    []string{"model", "knowledge"},
			"minimum_weknora_capability": "1", "license_id": licenseID,
			"change_notes": notes,
		},
	})
}

// submitForkVersion 提交并断言 201，返回解码后的 submission。
func submitForkVersion(t *testing.T, r *gin.Engine, role, actor, versionID, licenseID, version, notes string) forkLineageBody {
	t.Helper()
	submitted := submitForkVersionRaw(r, role, actor, versionID, licenseID, version, notes)
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var body forkLineageBody
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &body))
	return body
}

// approveForkSubmission 以 reviewer 身份批准，返回新 Release 的 id。
func approveForkSubmission(t *testing.T, r *gin.Engine, submissionID, digest string) string {
	t.Helper()
	approved := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions/"+submissionID+"/review", "admin", "reviewer", map[string]any{
		"expected_digest": digest, "decision": "approved",
	})
	require.Equal(t, http.StatusOK, approved.Code, approved.Body.String())
	var review forkReviewBody
	require.NoError(t, json.Unmarshal(approved.Body.Bytes(), &review))
	require.NotNil(t, review.Data.Release)
	return review.Data.Release.ID
}

// AC1：Mapping 修改不误判为 Fork；可移植核心修改判为 Fork（同链对照）；
// lineage 由服务端推导、请求体伪造被拒；原始内容无 lineage（Review Focus 3/4）。
func TestAgentForkLineageMappingEditsAreNotForks(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	registerForkLicense(t, r, "MIT", true)
	listingID, releaseID := publishAdoptionRelease(t, r)

	// 原始内容（agent-owned 的冻结版本，无 variant 台账）：无 lineage、
	// 不查许可证注册表（license 未注册仍 201）。
	origFrozen := publicCall(r, 1, false, http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
	require.Equal(t, http.StatusCreated, origFrozen.Code, origFrozen.Body.String())
	var origVersion struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(origFrozen.Body.Bytes(), &origVersion))
	original := submitForkVersion(t, r, "contributor", "contributor", origVersion.Data.ID, "unregistered-raw", "9.0.0", "")
	require.False(t, original.Data.IsFork)
	require.Empty(t, original.Data.ForkSourceListingID, "原始内容没有 lineage（Review Focus 4）")
	require.Empty(t, original.Data.LineageLicenseID)
	require.NotContains(t, string(original.Data.Manifest), `"lineage"`)

	// 纯映射派生（两次映射修改后直接提交发布冻结的版本）→ is_fork=false。
	_, mappingVersionID := seedForkChain(t, r, listingID, "")
	mapping := submitForkVersion(t, r, "admin", "admin", mappingVersionID, "MIT", "2.0.0", "re-mapped knowledge binding")
	require.False(t, mapping.Data.IsFork, "AC1：Mapping 修改不误判为 Fork")
	require.Equal(t, listingID, mapping.Data.ForkSourceListingID)
	require.Equal(t, releaseID, mapping.Data.ForkSourceReleaseID)
	require.Equal(t, "MIT", mapping.Data.LineageLicenseID)
	require.Equal(t, "re-mapped knowledge binding", mapping.Data.ForkNotes, "修改说明 = ChangeNotes")
	require.Contains(t, string(mapping.Data.Manifest), `"lineage"`)
	require.Contains(t, string(mapping.Data.Manifest), `"is_fork":false`)

	// 伪造 lineage 的请求体 → strict decode 400（Review Focus 3）。归因闭环
	// （修复轮 #62-R1）：metadata 完整合法（experts 的 manifest 校验不会独立
	// 拒绝它——若缺 DisallowUnknownFields，该 body 会被 decode 放行并成功
	// 提交），因此 400 只能来自 strict decode；响应消息点名 unknown field，
	// 同 body 去掉伪造字段后 201，进一步钉死 400 归因于伪造字段本身。
	forgedMetadata := map[string]any{
		"semantic_version": "2.0.5", "display_name": "Derived helper", "summary": "Derived",
		"supported_languages": []string{"en"}, "use_cases": []string{"support"},
		"capability_requirements":    []string{"model", "knowledge"},
		"minimum_weknora_capability": "1", "license_id": "MIT",
	}
	forge := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "admin", "admin", map[string]any{
		"agent_version_id":       mappingVersionID,
		"metadata":               forgedMetadata,
		"fork_source_release_id": "fake",
	})
	require.Equal(t, http.StatusBadRequest, forge.Code, forge.Body.String())
	// 响应体是 JSON，错误消息内的引号被序列化为 \"（反斜杠+引号）。
	require.Contains(t, forge.Body.String(), "json: unknown field \\\"fork_source_release_id\\\"", "400 归因于 strict decode 拒绝伪造的 lineage 字段")
	// 对照：同一 body 去掉伪造字段 → 201（证明除伪造字段外请求完全合法，
	// 上面的 400 不可能由空/非法 metadata 等其他校验产生）。
	honest := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/tenant/release-submissions", "admin", "admin", map[string]any{
		"agent_version_id": mappingVersionID, "metadata": forgedMetadata,
	})
	require.Equal(t, http.StatusCreated, honest.Code, honest.Body.String())

	// 对照：修改可移植核心 → is_fork=true。
	_, forkVersionID := seedForkChain(t, r, listingID, "Be portable, but sharper.")
	fork := submitForkVersion(t, r, "admin", "admin", forkVersionID, "MIT", "2.1.0", "sharper prompt")
	require.True(t, fork.Data.IsFork, "可移植核心修改 = Fork（对照）")
	require.Contains(t, string(fork.Data.Manifest), `"is_fork":true`)
}

// AC2（租户 lane）：来源许可证禁止再分发/未注册 → Submission 被服务端拒绝
// （409，消息点名许可证）；翻转注册表后同形态派生链 live 放行。
func TestAgentForkLineageRedistributionForbiddenRejectsSubmission(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	registerForkLicense(t, r, "tenant-private", false)

	// publishSourceThenFork：以给定许可证发布一条新源 Release（真实
	// 提交+审批，listing 复用累积 Release），跑完整派生链并提交 fork。
	// sourceSemVer 必须逐次递增（1.0.0/1.0.1/1.0.2）：同一 listing 的
	// agent_releases 受 uq_agent_releases_semantic(listing_id, semantic_version)
	// 唯一索引约束，源审批版本重复会使第二次审批撞唯一冲突而非 200。
	// fork 提交三次同为 "2.0.0" 无碍——前两次 409 未落任何行。
	publishSourceThenFork := func(metadataLicense, sourceSemVer, version string) *httptest.ResponseRecorder {
		frozen := publicCall(r, 1, false, http.MethodPost, "/api/v1/agents/agent-owned/versions", "contributor", "contributor", nil)
		require.Equal(t, http.StatusCreated, frozen.Code, frozen.Body.String())
		var sourceVersion struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(frozen.Body.Bytes(), &sourceVersion))
		source := submitForkVersion(t, r, "contributor", "contributor", sourceVersion.Data.ID, metadataLicense, sourceSemVer, "")
		approveForkSubmission(t, r, source.Data.ID, source.Data.BundleDigest)
		_, forkVersionID := seedForkChain(t, r, source.Data.ListingID, "Be portable, forked.")
		return submitForkVersionRaw(r, "admin", "admin", forkVersionID, metadataLicense, version, "forked")
	}

	// 已注册但禁止再分发 → 409，消息点名许可证。
	forbidden := publishSourceThenFork("tenant-private", "1.0.0", "2.0.0")
	require.Equal(t, http.StatusConflict, forbidden.Code, forbidden.Body.String())
	require.Contains(t, forbidden.Body.String(), "tenant-private")
	require.Contains(t, forbidden.Body.String(), "redistribution")

	// 未注册 → 409，fail closed。
	unregistered := publishSourceThenFork("ghost-license", "1.0.1", "2.0.0")
	require.Equal(t, http.StatusConflict, unregistered.Code, unregistered.Body.String())
	require.Contains(t, unregistered.Body.String(), "not registered")

	// 翻转注册表为允许 → 同形态派生链 live 放行，并如实记为 Fork。
	registerForkLicense(t, r, "tenant-private", true)
	allowed := publishSourceThenFork("tenant-private", "1.0.2", "2.0.0")
	require.Equal(t, http.StatusCreated, allowed.Code, allowed.Body.String())
	var allowedBody forkLineageBody
	require.NoError(t, json.Unmarshal(allowed.Body.Bytes(), &allowedBody))
	require.True(t, allowedBody.Data.IsFork)
}

// AC2（公共 lane）：放行期审批的派生 Release 可升公共提交；许可证翻转后，
// 同形态新 Release 的公共提交被 live 拒绝；恢复后放行——租户 lane 的历史
// 放行不被缓存。
func TestAgentForkLineagePublicSubmissionLicenseGate(t *testing.T) {
	r, _, _ := newPublicMarketplaceTestApp(t)
	registerForkLicense(t, r, "MIT", true)
	listingID, _ := publishAdoptionRelease(t, r)

	// F1：fork 链 → 提交 → 审批（放行期）。fork 提交的 SourceAgentID 是
	// variant 发布的本地 agent，CreateSubmission 按 (tenant_id,
	// source_agent_id) 复用 listing——fork Release 因此落在派生 agent 自己
	// 的 listing（Fork 作为独立 Agent Definition 维护，CONTEXT.md:134），
	// 而不是源 listing。公共提升必须用 fork submission 响应自身的
	// listing_id；许可证门读的是 release 行的 lineage_license_id，不受影响。
	_, forkOneVersion := seedForkChain(t, r, listingID, "Be portable, forked.")
	forkOne := submitForkVersion(t, r, "admin", "admin", forkOneVersion, "MIT", "2.0.0", "forked once")
	require.True(t, forkOne.Data.IsFork)
	require.NotEqual(t, listingID, forkOne.Data.ListingID, "fork Release 属于派生 agent 自己的 listing")
	forkOneReleaseID := approveForkSubmission(t, r, forkOne.Data.ID, forkOne.Data.BundleDigest)

	// F2：第二条 fork 链（仍在放行期完成租户 lane 提交与审批）。
	_, forkTwoVersion := seedForkChain(t, r, listingID, "Be portable, forked twice.")
	forkTwo := submitForkVersion(t, r, "admin", "admin", forkTwoVersion, "MIT", "3.0.0", "forked twice")
	require.True(t, forkTwo.Data.IsFork)
	forkTwoReleaseID := approveForkSubmission(t, r, forkTwo.Data.ID, forkTwo.Data.BundleDigest)

	// Verified Publisher 注册（SystemAdmin 面）。
	verified := publicCall(r, 1, true, http.MethodPost, "/api/v1/marketplace/public/verified-publishers", "admin", "sysadmin", map[string]any{"tenant_id": 1})
	require.Contains(t, []int{http.StatusOK, http.StatusCreated}, verified.Code, verified.Body.String())

	// 放行期：F1 升公共提交 → 201。
	promotedOne := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "admin", map[string]any{
		"source_listing_id": forkOne.Data.ListingID, "release_id": forkOneReleaseID,
	})
	require.Equal(t, http.StatusCreated, promotedOne.Code, promotedOne.Body.String())

	// 翻转：MIT 禁止再分发。
	registerForkLicense(t, r, "MIT", false)

	// F2 升公共提交 → 409（live 查询，历史放行不缓存）。
	promotedTwo := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "admin", map[string]any{
		"source_listing_id": forkTwo.Data.ListingID, "release_id": forkTwoReleaseID,
	})
	require.Equal(t, http.StatusConflict, promotedTwo.Code, promotedTwo.Body.String())
	require.Contains(t, promotedTwo.Body.String(), "redistribution")

	// 恢复允许后，同一 F2 放行（被拒的尝试未落任何行，可重试）。
	registerForkLicense(t, r, "MIT", true)
	retried := publicCall(r, 1, false, http.MethodPost, "/api/v1/marketplace/public/release-submissions", "admin", "admin", map[string]any{
		"source_listing_id": forkTwo.Data.ListingID, "release_id": forkTwoReleaseID,
	})
	require.Equal(t, http.StatusCreated, retried.Code, retried.Body.String())
}
