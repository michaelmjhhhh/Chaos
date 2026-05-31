# Ollama Provider and Settings Refresh Design

**Date:** 2026-05-31
**Status:** Approved by delegated product direction
**Project:** Chaos macOS screenshot organizer

## Goal

Add Ollama as a first-class optional local vision provider and make Settings
clearer when users choose a provider, enter credentials, or test a connection.

## Scope

This work includes:

- an `Ollama` provider preset that reuses the existing OpenAI-compatible vision
  request path;
- provider metadata for credential requirements, editable endpoints, and
  explanatory copy;
- provider-switch behavior that removes stale model and endpoint overrides;
- a visual refresh of the Settings window using the existing warm editorial
  design language;
- focused tests and README documentation.

Chaos will not install Ollama, download models, launch background services, or
manage the Ollama lifecycle. Users install Ollama and pull models separately.

## Product Decisions

### Ollama preset

- Add `Ollama` to the provider picker.
- Use `http://localhost:11434/v1` as its default base URL.
- Use `qwen3-vl:2b` as its default model. It is a lightweight vision-language
  model suited to short screenshot filename generation.
- Do not ask users for an API key when Ollama is selected.
- Internally pass an empty API key to the shared client. The authorization
  header remains harmless for the OpenAI-compatible Ollama endpoint and avoids
  branching the networking layer.

### Provider-switch behavior

- When the user selects a different provider, clear explicit `model` and
  `base_url` overrides before saving the new provider.
- Resolve the newly selected provider's defaults immediately.
- Preserve the configured API key when moving among providers. This avoids
  destructive credential loss during exploration. Ollama ignores the stored
  key and does not expose it in the interface.
- Keep `OpenAI-Compatible` as the only provider with an editable base URL.
  Preset providers show their resolved endpoint as read-only supporting text.

### Settings information architecture

Replace the dense grouped `Form` with a scrollable editorial layout:

1. A compact masthead introduces Settings as the place to connect a naming
   model and tune the filing workflow.
2. An `AI Provider` card contains:
   - provider picker;
   - a short provider-specific description;
   - a local or remote badge;
   - API key input only when required;
   - model input with the provider default as its prompt;
   - editable base URL only for `OpenAI-Compatible`;
   - read-only endpoint copy for preset providers;
   - connection test action and nearby result feedback.
3. `Directories`, `Output`, and `Behavior` cards retain existing controls with
   more visible grouping.
4. A quiet `Configuration` card shows the persisted config path and reveal
   action.

Use the existing cream canvas, white cards, coral primary action, subtle
borders, semantic status colors, system icons, and native macOS controls.

### Connection feedback

- Preserve the existing asynchronous connection test and disabled loading
  state.
- Show a labeled success or failure result next to the test action. Do not rely
  on color alone.
- When Ollama fails its health check, show:

  ```text
  Start Ollama, then run: ollama pull qwen3-vl:2b
  ```

- For remote providers, show a concise prompt to verify the API key and
  network connection.

## Architecture

### Provider metadata

Extend `Provider` with:

- the new `.ollama` case;
- `requiresAPIKey`;
- `allowsCustomBaseURL`;
- `connectionKind`;
- `summary`;
- `connectionFailureHint`.

Keep defaults and presentation metadata together because they describe each
provider preset and are consumed by both state resolution and Settings.

### App state

Add:

- `resolvedAPIKey`, which returns an empty string for Ollama and the configured
  key otherwise;
- `selectProvider(_:)`, which resets stale model and endpoint overrides;
- conditional validation in `start()` so only providers that require an API
  key block watcher startup when the key is missing.

Pass `resolvedAPIKey` through health checks and image processing.

### SwiftUI composition

Keep `SettingsView` responsible for state bindings and actions. Extract small
presentation components into `SettingsComponents.swift`:

- `SettingsCard`;
- `SettingsCardHeader`;
- `SettingsBadge`;
- `SettingsConnectionResult`.

The components remain stateless and reusable. Native controls remain native.

## Error Handling

- Ollama missing or stopped: connection test reports failure and shows the
  local setup hint.
- Ollama model missing: the same hint includes the exact `ollama pull` command.
- Remote key missing: watcher startup continues to report
  `API key not configured`.
- OpenAI-compatible endpoint missing: Settings shows an inline warning and the
  connection test fails without crashing.
- Provider switching never deletes the persisted API key.

## Testing

Add focused XCTest coverage for:

- Ollama default model, endpoint, local connection kind, and API-key behavior;
- OpenAI-compatible custom endpoint behavior;
- provider switching clearing stale model and endpoint overrides while
  retaining the API key;
- watcher startup with Ollama not being rejected for a missing API key;
- existing provider fallback behavior.

Run:

```bash
swift test
git diff --check
./build-app.sh
```

Launch the built app and inspect Settings for:

- clear visual grouping;
- correct API-key visibility while switching providers;
- Ollama endpoint and model defaults;
- OpenAI-compatible custom URL field visibility;
- readable success and failure states.
