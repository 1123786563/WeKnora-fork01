# T14 Pre-Diagnostic Boundary Trace — Task 1 Report

## Scope and checkpoint

Executed only Task 1 from `.superpowers/sdd/2026-09-29-craft-107-t14-pre-diagnostic-boundary-trace/task-1-brief.md` in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`.

- Starting HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`.
- Scope: fixed, flushed stage start/completion markers through diagnostic receipt publication; candidate builder pin updated to final `probe.py` SHA.
- No Docker/image build, browser invocation, network/policy/timeout change, issue action, staging, or commit occurred.
- Read the assigned plan, `CONTEXT.md`, `docs/adr/0004-task-is-session.md`, approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`, and the named independent stage-start and pin reviews before editing.

## Changes

Added twelve allowlisted stage boundaries to `probe.py`: Selenium imports, identity wait, preview listener setup, WebDriver session creation, initial CDP bootstrap, control server startup, preview navigation/readiness, initial UX capture, viewport interaction/capture, HTTP control, WebSocket control, and diagnostic receipt publication. Each uses the existing `_stage_timing` context manager, which flushes a fixed two-field start record before the operation and the existing fixed completion schema afterward. Barrier methods, operation order, browser/policy configuration, and existing timeouts were not changed.

Updated the startup-failure test to assert the new fixed stderr trace, added a focused test for all labels/schema/order and start-before-operation behavior, and updated the exact probe digest in the candidate builder and topology assertion. The source image and five preview digests are unchanged.

## Before / after SHA-256 snapshots

The before hashes were captured before edits. Exact pre-task source copies were reconstructed by reversing only this task's changes and verified against all four captured hashes. The corresponding task-only unified patch is included below.

| Owned file | Before | After |
|---|---|---|
| `deploy/craft/render-boundary/probe.py` | `6537b05abdcdd917a95b5ae65248b66e87ed717aa61ccab2788f805a6376157b` | `4b580323f086622ca6e4954f7c9ad347dc8a9ed11bcdb27566ec492a7ce8668f` |
| `deploy/craft/render-boundary/test_probe.py` | `6c3a967acc29fbb7c72b6d67b05f574cf0e93527ef60108b40e595029d798940` | `c170fc2ee57c4556e68554a37afa63beb290a58ad6a6d4b88794fa46c9828077` |
| `deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh` | `aca065850a0e49ab47e0675567066d5c9ee424a37438acbfd835ed5ed89403c0` | `8b504ec814310c212b64b342a44b4aa0f75003e710cceb9c341e91bb3f80e6c2` |
| `deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py` | `ff27bdb11593a95096e6ec93e43afec70d4d483fa54bfc147ff3acd94521cdc3` | `915638b0f60590ecfc509e92e8d2b658771f5300a316621307c074eb1328a0cf` |

The final `probe.py` SHA equals both the builder pin and topology-test pin.

## TDD and verification evidence

- RED: `python3 -m unittest test_probe.ProbeBarrierTests.test_pre_diagnostic_stage_trace_uses_fixed_start_and_completion_records -v` from `deploy/craft/render-boundary` failed because the required labels were not allowlisted yet.
- GREEN: the same focused test passed after instrumentation.
- Pin RED: after setting the topology assertion to the newly computed final renderer SHA but before changing the builder, `python3 -m unittest deploy.craft.render-boundary.test_build_volume_free_diagnostics_candidate.DiagnosticsCandidateBuildTopologyTest.test_helper_pins_source_and_overlays_only_reviewed_runtime_files -v` failed because the shell pin still held the old digest. It passed after the builder pin update.
- `python3 -m unittest test_probe.ProbeBarrierTests -v`: 14 tests passed.
- `python3 -m unittest test_probe.ChromeDriverVersionTests.test_run_startup_failure_emits_bounded_diagnostics_and_cleans_up -v`: passed; asserts exact start/completion label order on startup failure and retains bounded diagnostic checks.
- `python3 -m unittest test_probe -v`: 148 tests passed.
- `python3 -m unittest deploy.craft.render-boundary.test_build_volume_free_diagnostics_candidate -v`: 4 tests passed. This is the actual module/file spelling (`free`). The brief's `test_build_volume_freeze_diagnostics_candidate` command was also attempted and fails module import because no such test module exists.
- `python3 -m py_compile deploy/craft/render-boundary/probe.py deploy/craft/render-boundary/test_probe.py`: passed.
- `sh -n deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh`: passed.
- `git diff --check` on the four owned paths: passed. A separate Python scan checked trailing spaces and tabs in every line of all four paths, explicitly including the untracked builder topology test: all four clean.
- A full `python3 -m unittest test_probe -v` run initially exposed the existing startup test's stale empty-stderr expectation; that test now validates the exact new fixed trace. The subsequent full run passed all 148 tests.

No live runtime acceptance is claimed; the parent owns independent review/validation and any later bounded run.

## Task-only patch

```diff
--- a/deploy/craft/render-boundary/probe.py
+++ b/deploy/craft/render-boundary/probe.py
@@ -122,6 +122,11 @@
     "renderer_popup_target_after", "renderer_popup_switch_to", "renderer_popup_frame_tree",
     "renderer_popup_name", "renderer_popup_switch_back", "renderer_frame_tree",
     "renderer_attempt_begin_publish",
+    "renderer_selenium_imports", "renderer_identity_wait", "renderer_preview_listener_setup",
+    "renderer_webdriver_session", "renderer_initial_cdp_bootstrap", "renderer_control_server_start",
+    "renderer_preview_navigation_ready", "renderer_initial_ux_capture",
+    "renderer_viewport_interaction_capture", "renderer_http_control",
+    "renderer_websocket_control", "renderer_diagnostic_receipt_publish",
 })
 
 
@@ -3049,28 +3054,31 @@
 
 
 def run() -> dict[str, object]:
-    from selenium import webdriver
-    from selenium.webdriver import ActionChains
-    from selenium.webdriver.chrome.options import Options
-    from selenium.webdriver.chrome.service import Service
-    from selenium.webdriver.common.by import By
-    from selenium.webdriver.common.keys import Keys
-    from selenium.webdriver.support.ui import WebDriverWait
+    with _stage_timing("renderer_selenium_imports"):
+        from selenium import webdriver
+        from selenium.webdriver import ActionChains
+        from selenium.webdriver.chrome.options import Options
+        from selenium.webdriver.chrome.service import Service
+        from selenium.webdriver.common.by import By
+        from selenium.webdriver.common.keys import Keys
+        from selenium.webdriver.support.ui import WebDriverWait
 
     nonce = os.environ.get("CRAFT_T14_RUN_NONCE", "")
     barrier = ProbeBarrier(nonce)
-    barrier_identity = barrier.wait_for_identity()
-    Path(os.environ.get("HOME", "/tmp/renderer-home")).mkdir(mode=0o700, parents=True, exist_ok=True)
-    ensure_loopback_probe_port_unserved()
-    manifest = load_manifest(FIXTURE_ROOT, MAX_FILES, MAX_FILE_BYTES, MAX_TOTAL_BYTES)
-    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), ManifestHandler)
-    if server.server_port == 8081:
-        server.server_close()
-        raise RuntimeError("preview origin unexpectedly occupies loopback egress probe port 8081")
-    server.manifest = manifest  # type: ignore[attr-defined]
-    barrier.set_preview_port(server.server_port)
-    thread = threading.Thread(target=server.serve_forever, daemon=True)
-    thread.start()
+    with _stage_timing("renderer_identity_wait"):
+        barrier_identity = barrier.wait_for_identity()
+    with _stage_timing("renderer_preview_listener_setup"):
+        Path(os.environ.get("HOME", "/tmp/renderer-home")).mkdir(mode=0o700, parents=True, exist_ok=True)
+        ensure_loopback_probe_port_unserved()
+        manifest = load_manifest(FIXTURE_ROOT, MAX_FILES, MAX_FILE_BYTES, MAX_TOTAL_BYTES)
+        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), ManifestHandler)
+        if server.server_port == 8081:
+            server.server_close()
+            raise RuntimeError("preview origin unexpectedly occupies loopback egress probe port 8081")
+        server.manifest = manifest  # type: ignore[attr-defined]
+        barrier.set_preview_port(server.server_port)
+        thread = threading.Thread(target=server.serve_forever, daemon=True)
+        thread.start()
     browser = None
     control_server = None
     netlog_path = NETLOG_PATH
@@ -3097,191 +3105,199 @@
         options.add_argument("--net-log-capture-mode=Everything")
         chromium_launch_provenance = validate_chromium_public_mapping(options.arguments)
         options.set_capability("goog:loggingPrefs", {"performance": "ALL"})
-        try:
-            browser = webdriver.Chrome(service=_chrome_service(Service), options=options)
-        except Exception:
-            diagnostics = collect_chrome_startup_diagnostics(
-                chromedriver_log=CHROMEDRIVER_LOG_PATH,
-                chromium_log=CHROMIUM_LOG_PATH,
-            )
-            raise ChromeStartupFailure(
-                "Chrome session creation failed; startup diagnostics="
-                + json.dumps(diagnostics, separators=(",", ":"), sort_keys=True)
-            ) from None
-        browser.execute_cdp_cmd("Network.enable", {})
-        browser.execute_cdp_cmd("Page.enable", {})
-        browser.execute_cdp_cmd("Network.setCacheDisabled", {"cacheDisabled": True})
-        browser.set_page_load_timeout(8)
-        browser.set_script_timeout(5)
-        version = browser.capabilities.get("browserVersion", "unknown")
-        control_server, _control_thread = start_loopback_diagnostic_server(8081)
-        browser.get(f"http://127.0.0.1:{server.server_port}/?t14_ux_state=loading")
-        WebDriverWait(browser, 5).until(
-            lambda driver: driver.execute_script("return document.readyState") == "complete"
-            and driver.find_element(By.ID, "status").text == "loading"
+        with _stage_timing("renderer_webdriver_session"):
+            try:
+                browser = webdriver.Chrome(service=_chrome_service(Service), options=options)
+            except Exception:
+                diagnostics = collect_chrome_startup_diagnostics(
+                    chromedriver_log=CHROMEDRIVER_LOG_PATH,
+                    chromium_log=CHROMIUM_LOG_PATH,
+                )
+                raise ChromeStartupFailure(
+                    "Chrome session creation failed; startup diagnostics="
+                    + json.dumps(diagnostics, separators=(",", ":"), sort_keys=True)
+                ) from None
+        with _stage_timing("renderer_initial_cdp_bootstrap"):
+            browser.execute_cdp_cmd("Network.enable", {})
+            browser.execute_cdp_cmd("Page.enable", {})
+            browser.execute_cdp_cmd("Network.setCacheDisabled", {"cacheDisabled": True})
+            browser.set_page_load_timeout(8)
+            browser.set_script_timeout(5)
+            version = browser.capabilities.get("browserVersion", "unknown")
+        with _stage_timing("renderer_control_server_start"):
+            control_server, _control_thread = start_loopback_diagnostic_server(8081)
+        with _stage_timing("renderer_preview_navigation_ready"):
+            browser.get(f"http://127.0.0.1:{server.server_port}/?t14_ux_state=loading")
+            WebDriverWait(browser, 5).until(
+                lambda driver: driver.execute_script("return document.readyState") == "complete"
+                and driver.find_element(By.ID, "status").text == "loading"
         )
 
-        ux_states: dict[str, object] = {}
-        ux_incomplete_observations: list[dict[str, str]] = []
-        ux_target_id = browser.current_window_handle
-        for state in ("loading", "empty", "error", "success"):
-            if state != "loading" and browser.execute_script("return window.setPreviewFixtureState(arguments[0])", state) is not True:
-                raise ValueError("preview fixture rejected UX state: " + state)
-            state_start = time.time_ns()
-            status_text = browser.find_element(By.ID, "status").text
-            if status_text != ("ready" if state == "success" else state):
-                raise ValueError("preview did not visibly enter UX state: " + state)
-            state_visible = _try_ux_script(browser, """const state = arguments[0];
-              const id = {loading: 'status', empty: 'empty-message', error: 'error-message', success: 'success-message'}[state];
-              const element = document.getElementById(id), style = getComputedStyle(element), rect = element.getBoundingClientRect();
-              return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0;
-            """, bool, "state visibility", ux_incomplete_observations, state)
-            accessibility_tree = _try_ux_accessibility_tree(browser, "state accessibility tree", ux_incomplete_observations)
-            screenshot_state = _try_ux_screenshot(browser, "state screenshot", ux_incomplete_observations)
-            ux_states[state] = {
-                "status": status_text,
-                "state_visible": state_visible,
-                "viewport": _try_ux_script(browser, "return {width: innerWidth, height: innerHeight}", dict, "state viewport", ux_incomplete_observations),
-                "dom": _try_ux_script(browser, "return document.documentElement.outerHTML", str, "state DOM", ux_incomplete_observations),
-                "text": _try_ux_script(browser, "return document.body.innerText", str, "state text", ux_incomplete_observations),
-                "accessibility_tree": accessibility_tree,
-                "accessibility_names_roles": _accessibility_names_roles(accessibility_tree),
-                "screenshot_png_base64": base64.b64encode(screenshot_state).decode("ascii"),
-                "screenshot_sha256": hashlib.sha256(screenshot_state).hexdigest(),
-                "capture_started_unix_ns": state_start,
-                "capture_completed_unix_ns": time.time_ns(),
-                "target_id": ux_target_id,
+        with _stage_timing("renderer_initial_ux_capture"):
+            ux_states: dict[str, object] = {}
+            ux_incomplete_observations: list[dict[str, str]] = []
+            ux_target_id = browser.current_window_handle
+            for state in ("loading", "empty", "error", "success"):
+                if state != "loading" and browser.execute_script("return window.setPreviewFixtureState(arguments[0])", state) is not True:
+                    raise ValueError("preview fixture rejected UX state: " + state)
+                state_start = time.time_ns()
+                status_text = browser.find_element(By.ID, "status").text
+                if status_text != ("ready" if state == "success" else state):
+                    raise ValueError("preview did not visibly enter UX state: " + state)
+                state_visible = _try_ux_script(browser, """const state = arguments[0];
+                  const id = {loading: 'status', empty: 'empty-message', error: 'error-message', success: 'success-message'}[state];
+                  const element = document.getElementById(id), style = getComputedStyle(element), rect = element.getBoundingClientRect();
+                  return style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0;
+                """, bool, "state visibility", ux_incomplete_observations, state)
+                accessibility_tree = _try_ux_accessibility_tree(browser, "state accessibility tree", ux_incomplete_observations)
+                screenshot_state = _try_ux_screenshot(browser, "state screenshot", ux_incomplete_observations)
+                ux_states[state] = {
+                    "status": status_text,
+                    "state_visible": state_visible,
+                    "viewport": _try_ux_script(browser, "return {width: innerWidth, height: innerHeight}", dict, "state viewport", ux_incomplete_observations),
+                    "dom": _try_ux_script(browser, "return document.documentElement.outerHTML", str, "state DOM", ux_incomplete_observations),
+                    "text": _try_ux_script(browser, "return document.body.innerText", str, "state text", ux_incomplete_observations),
+                    "accessibility_tree": accessibility_tree,
+                    "accessibility_names_roles": _accessibility_names_roles(accessibility_tree),
+                    "screenshot_png_base64": base64.b64encode(screenshot_state).decode("ascii"),
+                    "screenshot_sha256": hashlib.sha256(screenshot_state).hexdigest(),
+                    "capture_started_unix_ns": state_start,
+                    "capture_completed_unix_ns": time.time_ns(),
+                    "target_id": ux_target_id,
+                }
+
+        with _stage_timing("renderer_viewport_interaction_capture"):
+            ux_viewports: dict[str, object] = {}
+            ux_focus_traversal: list[dict[str, object]] = []
+            try:
+                ux_interaction_mode_enabled = browser.execute_script("return window.enablePreviewUxInteractionMode()") is True
+            except Exception as exc:
+                ux_interaction_mode_enabled = False
+                ux_incomplete_observations.append({"observation": "viewport interaction mode", "reason": f"{type(exc).__name__}: {exc}"})
+            if not ux_interaction_mode_enabled:
+                ux_incomplete_observations.append({"observation": "viewport interaction mode", "reason": "fixture did not enable egress-suppressed interaction mode"})
+            for viewport_name, width, height in (("narrow", 375, 812), ("wide", 1280, 800)):
+                viewport_start = time.time_ns()
+                browser.execute_cdp_cmd("Emulation.setDeviceMetricsOverride", {
+                    "width": width, "height": height, "deviceScaleFactor": 1, "mobile": False,
+                })
+                browser.execute_script("document.body.tabIndex = -1; document.body.focus()")
+                try:
+                    viewport = _execute_ux_script(browser, """return (() => {
+                  const controls = ['status', 'click-me', 'key-target'].map(id => {
+                    const element = document.getElementById(id);
+                    const rect = element.getBoundingClientRect();
+                    const style = getComputedStyle(element);
+                    const visible = style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0;
+                    const x = rect.left + rect.width / 2, y = rect.top + rect.height / 2;
+                    const hit = document.elementFromPoint(x, y);
+                    return {id, tag: element.tagName.toLowerCase(), accessible_name: element.getAttribute('aria-label') ||
+                      (element.labels?.[0]?.innerText ?? element.innerText ?? element.textContent).trim(),
+                      visible, in_viewport: visible && rect.left >= 0 && rect.top >= 0 && rect.right <= innerWidth && rect.bottom <= innerHeight,
+                      rect: {x: rect.x, y: rect.y, width: rect.width, height: rect.height},
+                      hit_test_target: hit?.id || hit?.parentElement?.id || null};
+                  });
+                  const contrast = selector => {
+                    const element = document.querySelector(selector), style = getComputedStyle(element);
+                    return {foreground: style.color, background: style.backgroundColor === 'rgba(0, 0, 0, 0)' ? getComputedStyle(document.querySelector('main')).backgroundColor : style.backgroundColor};
+                  };
+                  return {width: innerWidth, height: innerHeight, document_width: document.documentElement.scrollWidth,
+                    body_width: document.body.scrollWidth, horizontal_overflow: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) > innerWidth,
+                    controls, contrast_samples: {body: contrast('body'), heading: contrast('h1'), button: contrast('#click-me'), input: contrast('#key-target')}};
+                })();""", dict, "viewport metrics")
+                except UxObservationIncomplete as exc:
+                    viewport = {"width": 0, "height": 0, "horizontal_overflow": True, "controls": [], "contrast_samples": {}}
+                    ux_incomplete_observations.append({"observation": exc.observation, "reason": exc.reason})
+                for _ in range(2):
+                    ActionChains(browser).send_keys(Keys.TAB).perform()
+                    try:
+                        focused = _execute_ux_script(browser, """return (() => {
+                      const element = document.activeElement, style = getComputedStyle(element);
+                      const role = element.getAttribute('role') || ({BUTTON: 'button', INPUT: 'textbox'}[element.tagName] || element.tagName.toLowerCase());
+                      const name = element.getAttribute('aria-label') || (element.labels?.[0]?.innerText ?? element.innerText ?? element.textContent).trim();
+                      return {id: element.id || null, role, name, visible_focus: element.matches(':focus-visible') && style.outlineStyle !== 'none' && parseFloat(style.outlineWidth) >= 2,
+                        outline: {style: style.outlineStyle, width: style.outlineWidth, color: style.outlineColor}};
+                    })();""", dict, "focus observation")
+                        ux_focus_traversal.append({"viewport": viewport_name, **focused, "target_id": ux_target_id, "observed_unix_ns": time.time_ns()})
+                    except UxObservationIncomplete as exc:
+                        ux_focus_traversal.append({"viewport": viewport_name, "target_id": ux_target_id, "incomplete": exc.reason})
+                        ux_incomplete_observations.append({"observation": exc.observation, "reason": exc.reason})
+
+                interaction: dict[str, object] = {}
+                try:
+                    if not ux_interaction_mode_enabled:
+                        raise UxObservationIncomplete("viewport interaction", "egress-suppressed interaction mode is unavailable")
+                    if browser.execute_script("return window.preparePreviewFixtureInteraction()") is not True:
+                        raise UxObservationIncomplete("viewport interaction", "fixture could not reset button/input state")
+                    browser.find_element(By.ID, "click-me").click()
+                    WebDriverWait(browser, 2).until(lambda driver: driver.find_element(By.ID, "click-count").text == "1")
+                    status_after_click = browser.find_element(By.ID, "status").text
+                    key_target = browser.find_element(By.ID, "key-target")
+                    key_target.send_keys("probe", Keys.ENTER)
+                    WebDriverWait(browser, 2).until(lambda driver: driver.find_element(By.ID, "status").text == "clicked:1;key:probe")
+                    interaction = {
+                        "method": "webdriver_click+send_keys", "target_id": ux_target_id,
+                        "click_control_id": "click-me", "input_control_id": "key-target",
+                        "click_count": browser.find_element(By.ID, "click-count").text,
+                        "status_after_click": status_after_click,
+                        "input_value": key_target.get_attribute("value"),
+                        "status_after_input": browser.find_element(By.ID, "status").text,
+                        "observed_unix_ns": time.time_ns(),
+                    }
+                except Exception as exc:
+                    ux_incomplete_observations.append({"observation": "viewport interaction", "reason": f"{type(exc).__name__}: {exc}"})
+                screenshot_viewport = _try_ux_screenshot(browser, "viewport screenshot", ux_incomplete_observations)
+                viewport_dom = _try_ux_script(browser, "return document.documentElement.outerHTML", str, "viewport DOM", ux_incomplete_observations)
+                viewport_text = _try_ux_script(browser, "return document.body.innerText", str, "viewport text", ux_incomplete_observations)
+                viewport_accessibility_tree = _try_ux_accessibility_tree(browser, "viewport accessibility tree", ux_incomplete_observations)
+                ux_viewports[viewport_name] = {
+                    **viewport, "interaction": interaction,
+                    "dom": viewport_dom,
+                    "text": viewport_text,
+                    "accessibility_tree": viewport_accessibility_tree,
+                    "accessibility_names_roles": _accessibility_names_roles(viewport_accessibility_tree),
+                    "capture_started_unix_ns": viewport_start,
+                    "capture_completed_unix_ns": time.time_ns(),
+                    "target_id": ux_target_id,
+                    "screenshot_png_base64": base64.b64encode(screenshot_viewport).decode("ascii"),
+                    "screenshot_sha256": hashlib.sha256(screenshot_viewport).hexdigest(),
+                }
+                if not isinstance(viewport, dict):
+                    ux_viewports[viewport_name]["controls"] = []
+                try:
+                    if browser.execute_script("return window.preparePreviewFixtureInteraction()") is not True:
+                        ux_incomplete_observations.append({"observation": "viewport reset", "reason": "fixture could not reset after measurement"})
+                except Exception as exc:
+                    ux_incomplete_observations.append({"observation": "viewport reset", "reason": f"{type(exc).__name__}: {exc}"})
+            browser.execute_cdp_cmd("Emulation.clearDeviceMetricsOverride", {})
+            ux_evidence = {
+                "browser_version": version, "target_id": ux_target_id, "states": ux_states,
+                "focus_traversal": ux_focus_traversal, "viewports": ux_viewports,
+                "incomplete_observations": ux_incomplete_observations,
             }
-
-        ux_viewports: dict[str, object] = {}
-        ux_focus_traversal: list[dict[str, object]] = []
-        try:
-            ux_interaction_mode_enabled = browser.execute_script("return window.enablePreviewUxInteractionMode()") is True
-        except Exception as exc:
-            ux_interaction_mode_enabled = False
-            ux_incomplete_observations.append({"observation": "viewport interaction mode", "reason": f"{type(exc).__name__}: {exc}"})
-        if not ux_interaction_mode_enabled:
-            ux_incomplete_observations.append({"observation": "viewport interaction mode", "reason": "fixture did not enable egress-suppressed interaction mode"})
-        for viewport_name, width, height in (("narrow", 375, 812), ("wide", 1280, 800)):
-            viewport_start = time.time_ns()
-            browser.execute_cdp_cmd("Emulation.setDeviceMetricsOverride", {
-                "width": width, "height": height, "deviceScaleFactor": 1, "mobile": False,
-            })
-            browser.execute_script("document.body.tabIndex = -1; document.body.focus()")
-            try:
-                viewport = _execute_ux_script(browser, """return (() => {
-              const controls = ['status', 'click-me', 'key-target'].map(id => {
-                const element = document.getElementById(id);
-                const rect = element.getBoundingClientRect();
-                const style = getComputedStyle(element);
-                const visible = style.display !== 'none' && style.visibility !== 'hidden' && rect.width > 0 && rect.height > 0;
-                const x = rect.left + rect.width / 2, y = rect.top + rect.height / 2;
-                const hit = document.elementFromPoint(x, y);
-                return {id, tag: element.tagName.toLowerCase(), accessible_name: element.getAttribute('aria-label') ||
-                  (element.labels?.[0]?.innerText ?? element.innerText ?? element.textContent).trim(),
-                  visible, in_viewport: visible && rect.left >= 0 && rect.top >= 0 && rect.right <= innerWidth && rect.bottom <= innerHeight,
-                  rect: {x: rect.x, y: rect.y, width: rect.width, height: rect.height},
-                  hit_test_target: hit?.id || hit?.parentElement?.id || null};
-              });
-              const contrast = selector => {
-                const element = document.querySelector(selector), style = getComputedStyle(element);
-                return {foreground: style.color, background: style.backgroundColor === 'rgba(0, 0, 0, 0)' ? getComputedStyle(document.querySelector('main')).backgroundColor : style.backgroundColor};
-              };
-              return {width: innerWidth, height: innerHeight, document_width: document.documentElement.scrollWidth,
-                body_width: document.body.scrollWidth, horizontal_overflow: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth) > innerWidth,
-                controls, contrast_samples: {body: contrast('body'), heading: contrast('h1'), button: contrast('#click-me'), input: contrast('#key-target')}};
-            })();""", dict, "viewport metrics")
-            except UxObservationIncomplete as exc:
-                viewport = {"width": 0, "height": 0, "horizontal_overflow": True, "controls": [], "contrast_samples": {}}
-                ux_incomplete_observations.append({"observation": exc.observation, "reason": exc.reason})
-            for _ in range(2):
-                ActionChains(browser).send_keys(Keys.TAB).perform()
-                try:
-                    focused = _execute_ux_script(browser, """return (() => {
-                  const element = document.activeElement, style = getComputedStyle(element);
-                  const role = element.getAttribute('role') || ({BUTTON: 'button', INPUT: 'textbox'}[element.tagName] || element.tagName.toLowerCase());
-                  const name = element.getAttribute('aria-label') || (element.labels?.[0]?.innerText ?? element.innerText ?? element.textContent).trim();
-                  return {id: element.id || null, role, name, visible_focus: element.matches(':focus-visible') && style.outlineStyle !== 'none' && parseFloat(style.outlineWidth) >= 2,
-                    outline: {style: style.outlineStyle, width: style.outlineWidth, color: style.outlineColor}};
-                })();""", dict, "focus observation")
-                    ux_focus_traversal.append({"viewport": viewport_name, **focused, "target_id": ux_target_id, "observed_unix_ns": time.time_ns()})
-                except UxObservationIncomplete as exc:
-                    ux_focus_traversal.append({"viewport": viewport_name, "target_id": ux_target_id, "incomplete": exc.reason})
-                    ux_incomplete_observations.append({"observation": exc.observation, "reason": exc.reason})
-
-            interaction: dict[str, object] = {}
-            try:
-                if not ux_interaction_mode_enabled:
-                    raise UxObservationIncomplete("viewport interaction", "egress-suppressed interaction mode is unavailable")
-                if browser.execute_script("return window.preparePreviewFixtureInteraction()") is not True:
-                    raise UxObservationIncomplete("viewport interaction", "fixture could not reset button/input state")
-                browser.find_element(By.ID, "click-me").click()
-                WebDriverWait(browser, 2).until(lambda driver: driver.find_element(By.ID, "click-count").text == "1")
-                status_after_click = browser.find_element(By.ID, "status").text
-                key_target = browser.find_element(By.ID, "key-target")
-                key_target.send_keys("probe", Keys.ENTER)
-                WebDriverWait(browser, 2).until(lambda driver: driver.find_element(By.ID, "status").text == "clicked:1;key:probe")
-                interaction = {
-                    "method": "webdriver_click+send_keys", "target_id": ux_target_id,
-                    "click_control_id": "click-me", "input_control_id": "key-target",
-                    "click_count": browser.find_element(By.ID, "click-count").text,
-                    "status_after_click": status_after_click,
-                    "input_value": key_target.get_attribute("value"),
-                    "status_after_input": browser.find_element(By.ID, "status").text,
-                    "observed_unix_ns": time.time_ns(),
-                }
-            except Exception as exc:
-                ux_incomplete_observations.append({"observation": "viewport interaction", "reason": f"{type(exc).__name__}: {exc}"})
-            screenshot_viewport = _try_ux_screenshot(browser, "viewport screenshot", ux_incomplete_observations)
-            viewport_dom = _try_ux_script(browser, "return document.documentElement.outerHTML", str, "viewport DOM", ux_incomplete_observations)
-            viewport_text = _try_ux_script(browser, "return document.body.innerText", str, "viewport text", ux_incomplete_observations)
-            viewport_accessibility_tree = _try_ux_accessibility_tree(browser, "viewport accessibility tree", ux_incomplete_observations)
-            ux_viewports[viewport_name] = {
-                **viewport, "interaction": interaction,
-                "dom": viewport_dom,
-                "text": viewport_text,
-                "accessibility_tree": viewport_accessibility_tree,
-                "accessibility_names_roles": _accessibility_names_roles(viewport_accessibility_tree),
-                "capture_started_unix_ns": viewport_start,
-                "capture_completed_unix_ns": time.time_ns(),
-                "target_id": ux_target_id,
-                "screenshot_png_base64": base64.b64encode(screenshot_viewport).decode("ascii"),
-                "screenshot_sha256": hashlib.sha256(screenshot_viewport).hexdigest(),
-            }
-            if not isinstance(viewport, dict):
-                ux_viewports[viewport_name]["controls"] = []
-            try:
-                if browser.execute_script("return window.preparePreviewFixtureInteraction()") is not True:
-                    ux_incomplete_observations.append({"observation": "viewport reset", "reason": "fixture could not reset after measurement"})
-            except Exception as exc:
-                ux_incomplete_observations.append({"observation": "viewport reset", "reason": f"{type(exc).__name__}: {exc}"})
-        browser.execute_cdp_cmd("Emulation.clearDeviceMetricsOverride", {})
-        ux_evidence = {
-            "browser_version": version, "target_id": ux_target_id, "states": ux_states,
-            "focus_traversal": ux_focus_traversal, "viewports": ux_viewports,
-            "incomplete_observations": ux_incomplete_observations,
-        }
-        ux_validation = validate_ux_evidence(ux_evidence)
-        diagnostic_phase_start = time.time_ns()
+            ux_validation = validate_ux_evidence(ux_evidence)
+            diagnostic_phase_start = time.time_ns()
         diagnostic_namespace = os.readlink("/proc/self/ns/net")
         control_http_url = f"http://127.0.0.1:8081{LoopbackDiagnosticHandler.HTTP_PATH}?control=loopback-v1"
         control_ws_url = f"ws://127.0.0.1:8081{LoopbackDiagnosticHandler.WEBSOCKET_PATH}?control=websocket-v1"
-        http_control_result = browser.execute_async_script(
-            """const done = arguments[arguments.length - 1];
-               fetch(arguments[0], {method: 'POST', body: 'craft-t14-loopback-control-v1'})
-                 .then(response => done({ok: response.status === 204, status: response.status}))
-                 .catch(error => done({ok: false, error: String(error)}));""",
-            control_http_url,
+        with _stage_timing("renderer_http_control"):
+            http_control_result = browser.execute_async_script(
+                """const done = arguments[arguments.length - 1];
+                   fetch(arguments[0], {method: 'POST', body: 'craft-t14-loopback-control-v1'})
+                     .then(response => done({ok: response.status === 204, status: response.status}))
+                     .catch(error => done({ok: false, error: String(error)}));""",
+                control_http_url,
         )
-        websocket_control_result = browser.execute_async_script(
-            """const done = arguments[arguments.length - 1];
-               const socket = new WebSocket(arguments[0]);
-               let settled = false;
-               const finish = value => { if (!settled) { settled = true; done(value); } };
-               socket.onopen = () => { finish({opened: true}); socket.close(); };
-               socket.onerror = () => finish({opened: false, error: 'WebSocket error'});
-               setTimeout(() => finish({opened: false, error: 'WebSocket control timed out'}), 4000);""",
-            control_ws_url,
+        with _stage_timing("renderer_websocket_control"):
+            websocket_control_result = browser.execute_async_script(
+                """const done = arguments[arguments.length - 1];
+                   const socket = new WebSocket(arguments[0]);
+                   let settled = false;
+                   const finish = value => { if (!settled) { settled = true; done(value); } };
+                   socket.onopen = () => { finish({opened: true}); socket.close(); };
+                   socket.onerror = () => finish({opened: false, error: 'WebSocket error'});
+                   setTimeout(() => finish({opened: false, error: 'WebSocket control timed out'}), 4000);""",
+                control_ws_url,
         )
         control_raw_entries: list[dict[str, object]] = []
         control_raw_state: dict[str, object] = {"complete": True, "encoded_bytes": 0, "dropped_entries": 0}
@@ -3308,9 +3324,10 @@
             and control_raw_state.get("complete") is True and control_raw_state.get("dropped_entries") == 0
             and diagnostic_controls["raw_cdp_capture"].get("complete") is True
         )
-        barrier_diagnostic_receipt = barrier.wait_for_diagnostic_release(
-            controls_complete=diagnostic_controls["controls_complete"], listener_closed=control_server is None,
-        )
+        with _stage_timing("renderer_diagnostic_receipt_publish"):
+            barrier_diagnostic_receipt = barrier.wait_for_diagnostic_release(
+                controls_complete=diagnostic_controls["controls_complete"], listener_closed=control_server is None,
+            )
         prepared_attempt = _prepare_post_release_first_attempt(browser, By, Keys, WebDriverWait, barrier)
         screenshot = prepared_attempt["screenshot"]
         page_assets = prepared_attempt["page_assets"]
--- a/deploy/craft/render-boundary/test_probe.py
+++ b/deploy/craft/render-boundary/test_probe.py
@@ -2094,6 +2094,56 @@
     NAMESPACE = "net:[4026532999]"
     TARGET_ID = "A" * 32
 
+    def test_pre_diagnostic_stage_trace_uses_fixed_start_and_completion_records(self) -> None:
+        required = [
+            "renderer_selenium_imports", "renderer_identity_wait",
+            "renderer_preview_listener_setup", "renderer_webdriver_session",
+            "renderer_initial_cdp_bootstrap", "renderer_control_server_start",
+            "renderer_preview_navigation_ready", "renderer_initial_ux_capture",
+            "renderer_viewport_interaction_capture", "renderer_http_control",
+            "renderer_websocket_control", "renderer_diagnostic_receipt_publish",
+        ]
+        self.assertTrue(set(required).issubset(probe._STAGE_TIMING_LABELS))
+        source = Path(probe.__file__).read_text(encoding="utf-8")
+        positions = [source.index(f'_stage_timing("{label}")') for label in required]
+        self.assertEqual(positions, sorted(positions))
+
+        events: list[str] = []
+        output = io.StringIO()
+        emit_start = probe._emit_stage_start
+        emit_timing = probe._emit_stage_timing
+
+        def start(label: str, *, stream: object = None) -> int:
+            events.append("start")
+            return emit_start(label, stream=stream)
+
+        def finish(label: str, started_ns: int, ended_ns: int, *, stream: object = None) -> None:
+            events.append("finish")
+            emit_timing(label, started_ns, ended_ns, stream=stream)
+
+        with patch.object(probe, "_emit_stage_start", side_effect=start), patch.object(
+            probe, "_emit_stage_timing", side_effect=finish
+        ), patch.object(probe.sys, "stderr", output):
+            with probe._stage_timing(required[0]):
+                events.append("operation")
+
+        self.assertEqual(events, ["start", "operation", "finish"])
+        records = [
+            (line.split("=", 1)[0], json.loads(line.split("=", 1)[1]))
+            for line in output.getvalue().splitlines()
+        ]
+        self.assertEqual([event for event, _ in records], ["T14_STAGE_START", "T14_STAGE_TIMING"])
+        self.assertEqual(set(records[0][1]), {"label", "started_monotonic_ns"})
+        self.assertEqual(records[0][1]["label"], required[0])
+        self.assertIs(type(records[0][1]["started_monotonic_ns"]), int)
+        self.assertEqual(set(records[1][1]), {
+            "label", "started_monotonic_ns", "ended_monotonic_ns", "duration_ns",
+        })
+        self.assertEqual(records[1][1]["label"], required[0])
+        self.assertTrue(all(type(records[1][1][key]) is int for key in (
+            "started_monotonic_ns", "ended_monotonic_ns", "duration_ns",
+        )))
+
     def test_stage_timing_emits_only_fixed_label_and_numeric_fields(self) -> None:
         output = io.StringIO()
         probe._emit_stage_timing("renderer_click", 10, 17, stream=output)
@@ -2609,7 +2659,20 @@
                 self.assertNotIn(secret, emitted + stderr)
             self.assertLess(len(emitted), 20_000)
             self.assertLess(len(stderr), 20_000)
-            self.assertEqual(stderr, "")
+            stage_records = [
+                (line.split("=", 1)[0], json.loads(line.split("=", 1)[1]))
+                for line in stderr.splitlines()
+            ]
+            labels = [record[1]["label"] for record in stage_records]
+            self.assertEqual(labels, [
+                "renderer_selenium_imports", "renderer_selenium_imports",
+                "renderer_identity_wait", "renderer_identity_wait",
+                "renderer_preview_listener_setup", "renderer_preview_listener_setup",
+                "renderer_webdriver_session", "renderer_webdriver_session",
+            ])
+            self.assertEqual([event for event, _ in stage_records], [
+                "T14_STAGE_START", "T14_STAGE_TIMING",
+            ] * 4)
             self.assertFalse(netlog.exists())
             self.assertFalse(driver_log.exists())
             self.assertFalse(chromium_log.exists())
--- a/deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh
+++ b/deploy/craft/render-boundary/build-volume-free-diagnostics-candidate.sh
@@ -4,7 +4,7 @@
 EXPECTED_SOURCE_ID=sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27
 overlay_sha256() {
     case "$1" in
-        probe.py) echo 6537b05abdcdd917a95b5ae65248b66e87ed717aa61ccab2788f805a6376157b ;;
+        probe.py) echo 4b580323f086622ca6e4954f7c9ad347dc8a9ed11bcdb27566ec492a7ce8668f ;;
         preview/index.html) echo edae16bb05baa12aca7b34b1d3fb1f32b3a861f03ab047fc5342a910f43d39f2 ;;
         preview/app.js) echo 83173b30c49d075d8d9d89a04fca66cc7bf9d45de077b28b2cc45f06166738ea ;;
         preview/style.css) echo 8ac438d4712d8147686c471f2d451ed681575ae4a678f9807daf7bf76d25b9d8 ;;
--- a/deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py
+++ b/deploy/craft/render-boundary/test_build_volume_free_diagnostics_candidate.py
@@ -12,7 +12,7 @@
 ROOT = Path(__file__).resolve().parent
 HELPER = ROOT / "build-volume-free-diagnostics-candidate.sh"
 SOURCE_ID = "sha256:7e3c24469815aaed751e89d2d9fb6ac3899b0bfa57a925befd333639aa622c27"
-REVIEWED_PROBE_SHA256 = "6537b05abdcdd917a95b5ae65248b66e87ed717aa61ccab2788f805a6376157b"
+REVIEWED_PROBE_SHA256 = "4b580323f086622ca6e4954f7c9ad347dc8a9ed11bcdb27566ec492a7ce8668f"
 OVERLAYS = (
     ("probe.py", "/opt/probe.py"),
     ("preview/index.html", "/opt/craft-preview/index.html"),
```

### Compact command output record

- RED renderer test: `Ran 1 test`; `FAILED (failures=1)`, failing assertion `required labels ⊆ _STAGE_TIMING_LABELS` (expected pre-implementation failure).
- RED pin test: `Ran 1 test`; `FAILED (failures=1)`, helper lacked expected final renderer SHA `4b580323...e8668f` while still pinned to the prior SHA (expected pre-pin-update failure).
- GREEN renderer focused suite: `Ran 14 tests in 0.712s`; `OK`.
- GREEN full renderer suite: `Ran 148 tests in 9.450s`; `OK`.
- GREEN candidate helper suite: `Ran 4 tests in 1.984s`; `OK`.
- Static checks: both commands exited `0`; whitespace checks reported no trailing whitespace in all four owned source/test/builder paths.
- The misspelled brief test module exited `1` with `ModuleNotFoundError`; the corresponding actual `...test_build_volume_free_diagnostics_candidate` module passed as listed above.

## SDD task state

Task 1 implementation checkpoint is complete and ready for parent-assigned independent validation and review. No downstream/live task was started.
