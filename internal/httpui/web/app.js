"use strict";

const state = { volumes: [], view: null, activeVolumeID: null, activePageID: null, dialogSubmit: null, toastTimer: null };
const $ = (selector) => document.querySelector(selector);
const elements = {
  volumeList: $("#volumeList"), volumeCount: $("#volumeCount"), empty: $("#emptyState"), active: $("#activeWorkspace"),
  title: $("#volumeTitle"), shelf: $("#shelfMark"), version: $("#volumeVersion"), edition: $("#editionNote"), badge: $("#stateBadge"),
  strip: $("#pageStrip"), pageCount: $("#pageCount"), imageStage: $("#imageStage"), folio: $("#activeFolio"), imageMeta: $("#imageMeta"),
  editor: $("#transcriptionEditor"), revision: $("#revisionLabel"), charCount: $("#charCount"), findings: $("#findingList"), findingCount: $("#findingCount"),
  checkSummary: $("#checkSummary"), checkContent: $("#checkContent"), actions: $("#workflowActions"), manifest: $("#manifestBand"),
  dialog: $("#formDialog"), dialogForm: $("#dialogForm"), dialogFields: $("#dialogFields"), dialogTitle: $("#dialogTitle"),
  dialogKicker: $("#dialogKicker"), dialogError: $("#dialogError"), dialogSubmit: $("#dialogSubmit"), toast: $("#toast")
};

const stateNames = { Draft: "草稿", Transcribing: "转录中", Checking: "检查中", NeedsCorrection: "待整改", ReadyForReview: "待复核", Frozen: "已冻结", Accessioned: "已入藏" };
const categoryNames = { MissingGlyph: "缺字", Variant: "异体字", LayoutBreak: "版面断裂", FolioAnomaly: "页码异常" };
const operationNames = { "volume.created": "建立数字化卷", "volume.metadata_updated": "更新书目信息", "page.registered": "登记扫描页", "page.metadata_updated": "修改页面叶号", "pages.reordered": "调整页序", "page.transcription_revised": "修订转录", "finding.added": "登记疑难", "finding.resolved": "处理疑难", "integrity.completed": "完成完整性检查", "volume.frozen": "冻结数字版本", "manifest.issued": "签发入藏清单" };

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>'"]/g, (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" })[character]);
}

function idempotencyKey(prefix) {
  const random = crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `${prefix}-${random}`;
}

async function api(path, options = {}) {
  const headers = new Headers(options.headers || {});
  if (options.body && !(options.body instanceof FormData)) headers.set("Content-Type", "application/json");
  const response = await fetch(path, { ...options, headers });
  let payload;
  try { payload = await response.json(); } catch { payload = null; }
  if (!response.ok) {
    const error = new Error(payload?.error?.message || `请求失败（${response.status}）`);
    error.code = payload?.error?.code;
    error.status = response.status;
    throw error;
  }
  return payload;
}

function toast(message) {
  clearTimeout(state.toastTimer);
  elements.toast.textContent = message;
  elements.toast.classList.add("show");
  state.toastTimer = setTimeout(() => elements.toast.classList.remove("show"), 2400);
}

function setSaving(active) { $("#saveStatus").textContent = active ? "正在提交……" : "数据已同步"; }

async function loadVolumes(selectID) {
  const payload = await api("/api/volumes");
  state.volumes = payload.data || [];
  elements.volumeCount.textContent = `${state.volumes.length} 卷`;
  renderVolumeList();
  const preferred = selectID || state.activeVolumeID || state.volumes[0]?.id;
  if (preferred) await selectVolume(preferred);
  else { elements.empty.hidden = false; elements.active.hidden = true; }
}

function renderVolumeList() {
  const query = $("#volumeSearch").value.trim().toLowerCase();
  const filtered = state.volumes.filter((volume) => `${volume.title} ${volume.shelfMark}`.toLowerCase().includes(query));
  elements.volumeList.innerHTML = filtered.map((volume) => `<button class="volume-item ${volume.id === state.activeVolumeID ? "active" : ""}" type="button" data-volume-id="${escapeHTML(volume.id)}"><strong>${escapeHTML(volume.title)}</strong><span>${escapeHTML(volume.shelfMark)} · ${stateNames[volume.state] || volume.state} · ${volume.pageCount} 页</span></button>`).join("") || `<p class="empty-copy">没有匹配的卷</p>`;
}

async function selectVolume(volumeID) {
  state.activeVolumeID = volumeID;
  const payload = await api(`/api/volumes/${encodeURIComponent(volumeID)}`);
  state.view = payload.data;
  const pages = state.view.orderedPages || [];
  if (!pages.some((page) => page.id === state.activePageID)) state.activePageID = pages[0]?.id || null;
  renderVolumeList();
  renderWorkbench();
  location.hash = volumeID;
}

function renderWorkbench() {
  const view = state.view;
  const volume = view.volume;
  elements.empty.hidden = true;
  elements.active.hidden = false;
  elements.title.textContent = volume.title;
  elements.shelf.textContent = volume.shelfMark;
  elements.version.textContent = `版本 ${volume.version}`;
  elements.edition.textContent = volume.editionNote || "未填写版本说明";
  elements.badge.textContent = stateNames[volume.state] || volume.state;
  elements.badge.dataset.state = volume.state;
  renderPages();
  renderFindings();
  renderCheck();
  renderActions();
  renderManifest();
}

function renderPages() {
  const pages = state.view.orderedPages || [];
  const editable = !["Frozen", "Accessioned", "Checking"].includes(state.view.volume.state);
  elements.pageCount.textContent = `${pages.length} 页`;
  elements.strip.innerHTML = pages.map((page, index) => `<div class="page-sequence-item" style="display:flex;gap:3px"><button class="page-card ${page.id === state.activePageID ? "active" : ""}" type="button" role="option" aria-selected="${page.id === state.activePageID}" data-page-id="${escapeHTML(page.id)}"><img src="/api/pages/${encodeURIComponent(page.id)}/image" alt="${escapeHTML(page.folioLabel)} 扫描缩略图"><span>${escapeHTML(page.folioLabel || `第 ${index + 1} 页`)}</span></button>${editable ? `<span class="sequence-controls"><button type="button" data-move="-1" data-page-id="${escapeHTML(page.id)}" aria-label="前移 ${escapeHTML(page.folioLabel)}" ${index === 0 ? "disabled" : ""}><img src="/assets/icons/arrow-left.svg" alt=""></button><button type="button" data-move="1" data-page-id="${escapeHTML(page.id)}" aria-label="后移 ${escapeHTML(page.folioLabel)}" ${index === pages.length - 1 ? "disabled" : ""}><img src="/assets/icons/arrow-right.svg" alt=""></button></span>` : ""}</div>`).join("") || `<p class="empty-copy">上传扫描页后开始逐页转录。</p>`;
  renderActivePage();
}

function activePage() { return (state.view?.orderedPages || []).find((page) => page.id === state.activePageID); }

function renderActivePage() {
  const page = activePage();
  const locked = ["Frozen", "Accessioned", "Checking"].includes(state.view.volume.state);
  if (!page) {
    elements.imageStage.innerHTML = "<p>选择扫描页后查看影像</p>";
    elements.folio.textContent = "未选页";
    elements.imageMeta.textContent = "";
    elements.editor.value = "";
		elements.revision.textContent = "修订 0";
		elements.charCount.textContent = "0 字";
		elements.editor.disabled = true;
		$("#saveTranscriptionButton").disabled = true;
		$("#addFindingButton").disabled = true;
		$("#editPageButton").disabled = true;
		return;
  }
  elements.folio.textContent = page.folioLabel || "未标叶号";
  elements.imageMeta.textContent = `${page.width} × ${page.height} · ${formatBytes(page.byteSize)}`;
  elements.imageStage.innerHTML = `<img src="/api/pages/${encodeURIComponent(page.id)}/image" alt="${escapeHTML(page.folioLabel)} 扫描图像">`;
  elements.editor.value = page.transcription || "";
  elements.editor.disabled = locked;
  elements.revision.textContent = `修订 ${page.revision}`;
  elements.charCount.textContent = `${Array.from(elements.editor.value).length} 字`;
	$("#saveTranscriptionButton").disabled = locked;
	$("#addFindingButton").disabled = locked;
	$("#editPageButton").disabled = locked;
}

function formatBytes(bytes) { return bytes > 1048576 ? `${(bytes / 1048576).toFixed(1)} MiB` : `${Math.max(1, Math.round(bytes / 1024))} KiB`; }

function renderFindings() {
  const findings = state.view.volume.findings || [];
  const open = findings.filter((finding) => finding.status === "Open");
  elements.findingCount.textContent = `${open.length} 项未决`;
  elements.findings.innerHTML = open.map((finding) => `<div class="finding-row"><span class="finding-code">${escapeHTML(categoryNames[finding.category] || finding.category)}</span><div><strong>${escapeHTML(finding.location)}</strong><p>${escapeHTML(finding.observedText || "未记录原文")} ${finding.proposedText ? `→ ${escapeHTML(finding.proposedText)}` : ""}</p></div><button class="resolve-button" type="button" data-resolve-id="${escapeHTML(finding.id)}">处理</button></div>`).join("") || `<p class="empty-copy">当前没有未决疑难。</p>`;
}

function renderCheck() {
  const check = state.view.latestCheck;
  if (!check) { elements.checkSummary.textContent = "尚未检查"; elements.checkContent.innerHTML = `<p class="empty-copy">完成页面转录与疑难处理后运行确定性检查。</p>`; return; }
  const blockers = check.violations.filter((violation) => violation.severity === "Blocker").length;
  elements.checkSummary.textContent = check.status === "Passed" ? `第 ${check.runNumber} 次 · 已通过` : `第 ${check.runNumber} 次 · ${blockers} 项阻断`;
  elements.checkContent.innerHTML = check.violations.map((violation) => `<div class="violation-row"><span class="severity">${violation.severity === "Blocker" ? "阻断" : "提示"}</span><div><strong>${escapeHTML(violation.folioLabel || violation.code)}</strong><p>${escapeHTML(violation.message)}</p></div></div>`).join("") || `<p class="empty-copy">检查未发现阻断项，数字版本可以提交冻结。</p>`;
}

function renderActions() {
  const volume = state.view.volume;
  const buttons = [];
  if (["Transcribing", "NeedsCorrection", "ReadyForReview"].includes(volume.state)) buttons.push(`<button class="secondary-button" type="button" data-workflow="check">运行检查</button>`);
  if (volume.state === "ReadyForReview") buttons.push(`<button class="primary-button" type="button" data-workflow="freeze">冻结版本</button>`);
  if (volume.state === "Frozen") buttons.push(`<button class="primary-button" type="button" data-workflow="accession">签发入藏</button>`);
  elements.actions.innerHTML = buttons.join("");
  $("#uploadButton").disabled = ["Frozen", "Accessioned", "Checking"].includes(volume.state);
  $("#editMetadataButton").disabled = ["Frozen", "Accessioned", "Checking"].includes(volume.state);
}

function renderManifest() {
  const manifest = state.view.volume.manifest;
  elements.manifest.hidden = !manifest;
  if (!manifest) return;
  $("#manifestNumber").textContent = manifest.manifestNumber;
  $("#manifestMeta").textContent = `${manifest.reviewer} 复核 · ${new Date(manifest.issuedAt).toLocaleString("zh-CN")}`;
  $("#manifestDigest").textContent = manifest.frozenDigest;
  $("#manifestFingerprint").textContent = state.view.manifestDigest;
  $("#manifestValidity").textContent = state.view.manifestValid ? "摘要验证通过" : "摘要验证失败";
}

function field(name, label, value = "", type = "text", extra = "") {
  if (type === "textarea") return `<label class="field"><span>${label}</span><textarea name="${name}" ${extra}>${escapeHTML(value)}</textarea></label>`;
  return `<label class="field"><span>${label}</span><input name="${name}" type="${type}" value="${escapeHTML(value)}" ${extra}></label>`;
}

function openDialog({ kicker, title, fields, submitText = "确认", onSubmit }) {
  elements.dialogKicker.textContent = kicker;
  elements.dialogTitle.textContent = title;
  elements.dialogFields.innerHTML = fields;
  elements.dialogSubmit.textContent = submitText;
  elements.dialogError.textContent = "";
  state.dialogSubmit = onSubmit;
  elements.dialog.showModal();
  elements.dialog.querySelector("input, textarea, select")?.focus();
}

function formObject(form) { return Object.fromEntries(new FormData(form).entries()); }

function openCreateDialog() {
  openDialog({ kicker: "书目登记", title: "建立数字化卷", fields: field("title", "题名", "", "text", "required maxlength=200") + field("editionNote", "版本说明", "", "textarea") + field("shelfMark", "架藏号", "", "text", "required maxlength=100") + field("actor", "编目人员", "", "text", "required"), submitText: "建立卷", onSubmit: async (data) => {
    const result = await api("/api/volumes", { method: "POST", headers: { "Idempotency-Key": idempotencyKey("create") }, body: JSON.stringify(data) });
    await loadVolumes(result.data.id); toast("数字化卷已建立");
  }});
}

function openMetadataDialog() {
  const volume = state.view.volume;
  openDialog({ kicker: "书目维护", title: "编辑卷信息", fields: field("title", "题名", volume.title, "text", "required") + field("editionNote", "版本说明", volume.editionNote, "textarea") + field("shelfMark", "架藏号", volume.shelfMark, "text", "required") + field("actor", "修改人", "", "text", "required"), submitText: "保存信息", onSubmit: async (data) => {
    data.expectedVersion = volume.version;
    await mutate(`/api/volumes/${volume.id}`, "PATCH", data, "metadata"); toast("书目信息已更新");
  }});
}

function openUploadDialog() {
  openDialog({ kicker: "扫描页登记", title: "上传扫描图像", fields: field("folioLabel", "叶号", "", "text", "required placeholder=\"例如：卷一·一叶正\"") + field("image", "扫描图像", "", "file", "required accept=\"image/png,image/jpeg,image/gif\"") + field("actor", "登记人", "", "text", "required"), submitText: "上传并登记", onSubmit: async (data, form) => {
    const body = new FormData(form);
    body.set("expectedVersion", state.view.volume.version);
    const payload = await api(`/api/volumes/${state.activeVolumeID}/pages`, { method: "POST", headers: { "Idempotency-Key": idempotencyKey("upload") }, body });
    await selectVolume(payload.data.id); state.activePageID = payload.data.pages.at(-1)?.id; renderWorkbench(); toast("扫描页已登记");
  }});
}

function openFindingDialog() {
  const page = activePage(); if (!page) return;
  const options = Object.entries(categoryNames).map(([value, label]) => `<option value="${value}">${label}</option>`).join("");
  openDialog({ kicker: page.folioLabel, title: "登记校勘疑难", fields: `<label class="field"><span>类别</span><select name="category">${options}</select></label>` + field("location", "位置", "", "text", "required placeholder=\"例如：第 3 行第 7 字\"") + field("observedText", "所见文字") + field("proposedText", "拟定文字") + field("actor", "登记人", "", "text", "required"), submitText: "加入队列", onSubmit: async (data) => {
    data.expectedVersion = state.view.volume.version;
    await mutate(`/api/volumes/${state.activeVolumeID}/pages/${page.id}/findings`, "POST", data, "finding"); toast("疑难已加入队列");
  }});
}

function openPageMetadataDialog() {
	const page = activePage(); if (!page) return;
	openDialog({ kicker: "页面编目", title: "修改叶号", fields: field("folioLabel", "叶号", page.folioLabel, "text", "required maxlength=100") + field("actor", "修改人", "", "text", "required"), submitText: "保存叶号", onSubmit: async (data) => {
		data.expectedVersion = state.view.volume.version;
		await mutate(`/api/volumes/${state.activeVolumeID}/pages/${page.id}`, "PATCH", data, "page-metadata"); toast("页面叶号已更新");
	}});
}

function openResolveDialog(findingID) {
  const finding = state.view.volume.findings.find((item) => item.id === findingID);
  openDialog({ kicker: categoryNames[finding.category], title: `处理 ${finding.location}`, fields: field("resolution", "取舍依据", "", "textarea", "required") + field("resolvedBy", "处理人", "", "text", "required"), submitText: "确认处理", onSubmit: async (data) => {
    data.expectedVersion = state.view.volume.version;
    await mutate(`/api/volumes/${state.activeVolumeID}/findings/${findingID}/resolve`, "POST", data, "resolve"); toast("疑难处理已留痕");
  }});
}

function openAccessionDialog() {
  openDialog({ kicker: "人工复核", title: "签发入藏清单", fields: field("reviewer", "入藏复核人", "", "text", "required") + field("reviewNote", "复核说明", "", "textarea"), submitText: "签发入藏", onSubmit: async (data) => {
    data.expectedVersion = state.view.volume.version;
    await mutate(`/api/volumes/${state.activeVolumeID}/accession`, "POST", data, "accession"); toast("入藏清单已签发");
  }});
}

async function mutate(path, method, data, prefix) {
  setSaving(true);
  try {
    const payload = await api(path, { method, headers: { "Idempotency-Key": idempotencyKey(prefix) }, body: JSON.stringify(data) });
    await loadVolumes(payload.data.id);
    return payload;
  } finally { setSaving(false); }
}

async function reorder(pageID, direction) {
  const order = state.view.orderedPages.map((page) => page.id);
  const index = order.indexOf(pageID); const target = index + Number(direction);
  if (index < 0 || target < 0 || target >= order.length) return;
  [order[index], order[target]] = [order[target], order[index]];
  await mutate(`/api/volumes/${state.activeVolumeID}/reorder`, "POST", { expectedVersion: state.view.volume.version, pageOrder: order, actor: "工作台用户" }, "reorder");
  toast("页序已调整");
}

async function runWorkflow(action) {
  if (action === "accession") { openAccessionDialog(); return; }
  const path = action === "check" ? "checks" : "freeze";
  const actor = action === "check" ? "校勘人员" : "校勘负责人";
  await mutate(`/api/volumes/${state.activeVolumeID}/${path}`, "POST", { expectedVersion: state.view.volume.version, actor }, action);
  toast(action === "check" ? "完整性检查已完成" : "数字版本已冻结");
}

async function openAudit() {
  if (!state.activeVolumeID) return;
  const payload = await api(`/api/volumes/${state.activeVolumeID}/audit`);
  $("#auditList").innerHTML = payload.data.slice().reverse().map((event) => `<li><strong>${escapeHTML(operationNames[event.operation] || event.operation)}</strong><span>${escapeHTML(event.actor || "系统")} · 版本 ${event.version} · ${new Date(event.occurredAt).toLocaleString("zh-CN")}</span></li>`).join("");
  $("#auditDrawer").classList.add("open"); $("#auditDrawer").setAttribute("aria-hidden", "false"); $("#drawerScrim").hidden = false;
}

function closeAudit() { $("#auditDrawer").classList.remove("open"); $("#auditDrawer").setAttribute("aria-hidden", "true"); $("#drawerScrim").hidden = true; }

elements.dialogForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!state.dialogSubmit || !elements.dialogForm.reportValidity()) return;
  elements.dialogSubmit.disabled = true; elements.dialogError.textContent = "";
  try { await state.dialogSubmit(formObject(elements.dialogForm), elements.dialogForm); elements.dialog.close(); }
  catch (error) { elements.dialogError.textContent = error.message; if (error.code === "conflict") await selectVolume(state.activeVolumeID); }
  finally { elements.dialogSubmit.disabled = false; }
});

elements.volumeList.addEventListener("click", (event) => { const button = event.target.closest("[data-volume-id]"); if (button) selectVolume(button.dataset.volumeId).catch(showError); });
elements.strip.addEventListener("click", (event) => { const move = event.target.closest("[data-move]"); if (move) { reorder(move.dataset.pageId, move.dataset.move).catch(showError); return; } const button = event.target.closest("[data-page-id]"); if (button) { state.activePageID = button.dataset.pageId; renderPages(); } });
elements.findings.addEventListener("click", (event) => { const button = event.target.closest("[data-resolve-id]"); if (button) openResolveDialog(button.dataset.resolveId); });
elements.actions.addEventListener("click", (event) => { const button = event.target.closest("[data-workflow]"); if (button) runWorkflow(button.dataset.workflow).catch(showError); });
elements.editor.addEventListener("input", () => { elements.charCount.textContent = `${Array.from(elements.editor.value).length} 字`; });
$("#saveTranscriptionButton").addEventListener("click", async () => { const page = activePage(); if (!page) return; try { await mutate(`/api/volumes/${state.activeVolumeID}/pages/${page.id}/transcription`, "PUT", { expectedVersion: state.view.volume.version, transcription: elements.editor.value, actor: "转录人员" }, "transcription"); toast("转录已保存"); } catch (error) { showError(error); } });
$("#newVolumeButton").addEventListener("click", openCreateDialog);
document.querySelector("[data-action='new-volume']").addEventListener("click", openCreateDialog);
$("#editMetadataButton").addEventListener("click", openMetadataDialog);
$("#uploadButton").addEventListener("click", openUploadDialog);
$("#editPageButton").addEventListener("click", openPageMetadataDialog);
$("#addFindingButton").addEventListener("click", openFindingDialog);
$("#auditButton").addEventListener("click", () => openAudit().catch(showError));
$("#closeAuditButton").addEventListener("click", closeAudit); $("#drawerScrim").addEventListener("click", closeAudit);
$("#volumeSearch").addEventListener("input", renderVolumeList);
document.addEventListener("keydown", (event) => { if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s" && !elements.editor.disabled) { event.preventDefault(); $("#saveTranscriptionButton").click(); } if (event.key === "Escape") closeAudit(); });

function showError(error) { toast(error.message || "操作失败"); if (error.code === "conflict" && state.activeVolumeID) selectVolume(state.activeVolumeID).catch(() => {}); }

loadVolumes(location.hash.slice(1) || null).catch(showError);
