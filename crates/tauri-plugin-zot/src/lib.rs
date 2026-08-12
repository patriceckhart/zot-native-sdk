use serde::Deserialize;
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    path::PathBuf,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc,
    },
};
use tauri::{Emitter, Manager, Runtime};
use tokio::{
    io::{AsyncBufReadExt, AsyncWriteExt, BufReader},
    process::{ChildStdin, Command},
    sync::{oneshot, Mutex},
};

struct Bridge {
    stdin: Mutex<ChildStdin>,
    next_id: AtomicU64,
    pending: Mutex<HashMap<u64, oneshot::Sender<Result<Value, String>>>>,
}

impl Bridge {
    async fn call(&self, method: &str, mut body: Value) -> Result<Value, String> {
        let id = self.next_id.fetch_add(1, Ordering::Relaxed);
        body["id"] = json!(id);
        body["method"] = json!(method);
        let mut line = serde_json::to_vec(&body).map_err(|e| e.to_string())?;
        line.push(b'\n');
        let (tx, rx) = oneshot::channel();
        self.pending.lock().await.insert(id, tx);
        if let Err(error) = self.stdin.lock().await.write_all(&line).await {
            self.pending.lock().await.remove(&id);
            return Err(error.to_string());
        }
        rx.await.map_err(|_| "zot bridge closed".to_string())?
    }
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Options {
    provider: String,
    api_key: Option<String>,
    access_token: Option<String>,
    account_id: Option<String>,
    model: Option<String>,
    system_prompt: Option<String>,
}

#[tauri::command]
async fn create_session(
    bridge: tauri::State<'_, Arc<Bridge>>,
    options: Options,
) -> Result<String, String> {
    let id = uuid::Uuid::new_v4().to_string();
    bridge.call("create_session", json!({"session_id":id,"provider":options.provider,"api_key":options.api_key,"access_token":options.access_token,"account_id":options.account_id,"model":options.model,"system_prompt":options.system_prompt})).await?;
    Ok(id)
}
#[tauri::command]
async fn prompt(
    bridge: tauri::State<'_, Arc<Bridge>>,
    session_id: String,
    message: String,
) -> Result<(), String> {
    bridge
        .call("prompt", json!({"session_id":session_id,"message":message}))
        .await?;
    Ok(())
}
#[tauri::command]
async fn abort(bridge: tauri::State<'_, Arc<Bridge>>, session_id: String) -> Result<(), String> {
    bridge
        .call("abort", json!({"session_id":session_id}))
        .await?;
    Ok(())
}
#[tauri::command]
async fn close_session(
    bridge: tauri::State<'_, Arc<Bridge>>,
    session_id: String,
) -> Result<(), String> {
    bridge
        .call("close_session", json!({"session_id":session_id}))
        .await?;
    Ok(())
}
#[tauri::command]
async fn export_history(
    bridge: tauri::State<'_, Arc<Bridge>>,
    session_id: String,
) -> Result<String, String> {
    let value = bridge
        .call("export_history", json!({"session_id":session_id}))
        .await?;
    Ok(value["history"].as_str().unwrap_or_default().to_string())
}
#[tauri::command]
async fn import_history(
    bridge: tauri::State<'_, Arc<Bridge>>,
    session_id: String,
    history: String,
) -> Result<(), String> {
    bridge
        .call(
            "import_history",
            json!({"session_id":session_id,"history":history}),
        )
        .await?;
    Ok(())
}
#[tauri::command]
async fn extract_openai_account_id(
    bridge: tauri::State<'_, Arc<Bridge>>,
    id_token: String,
) -> Result<String, String> {
    let v = bridge
        .call("extract_openai_account_id", json!({"id_token":id_token}))
        .await?;
    Ok(v["account_id"].as_str().unwrap_or_default().to_string())
}

pub fn init<R: Runtime>(binary: PathBuf) -> tauri::plugin::TauriPlugin<R> {
    tauri::plugin::Builder::new("zot")
        .setup(move |app, _| {
            let mut child = Command::new(&binary)
                .stdin(std::process::Stdio::piped())
                .stdout(std::process::Stdio::piped())
                .spawn()
                .map_err(|e| e.to_string())?;
            let stdin = child.stdin.take().ok_or("missing bridge stdin")?;
            let stdout = child.stdout.take().ok_or("missing bridge stdout")?;
            let bridge = Arc::new(Bridge {
                stdin: Mutex::new(stdin),
                next_id: AtomicU64::new(1),
                pending: Mutex::new(HashMap::new()),
            });
            app.manage(bridge.clone());
            tauri::async_runtime::spawn(async move {
                let _ = child.wait().await;
            });
            let handle = app.clone();
            tauri::async_runtime::spawn(async move {
                let mut lines = BufReader::new(stdout).lines();
                while let Ok(Some(line)) = lines.next_line().await {
                    let Ok(value) = serde_json::from_str::<Value>(&line) else {
                        continue;
                    };
                    if let Some(id) = value["id"].as_u64() {
                        if let Some(tx) = bridge.pending.lock().await.remove(&id) {
                            let _ = tx.send(if let Some(error) = value["error"].as_str() {
                                Err(error.to_string())
                            } else {
                                Ok(value["result"].clone())
                            });
                        }
                    } else if let Some(kind) = value["event"].as_str() {
                        let event = match kind {
                            "text" => json!({"type":"text","delta":value["payload"]["delta"]}),
                            "error" => {
                                json!({"type":"error","message":value["payload"]["message"]})
                            }
                            "history" => {
                                json!({"type":"history","history":value["payload"]["history"]})
                            }
                            "done" => json!({"type":"done"}),
                            _ => json!({"type":kind,"payload":value["payload"]}),
                        };
                        let _ = handle.emit(
                            "zot:event",
                            json!({"sessionId":value["session_id"],"event":event}),
                        );
                    }
                }
                for (_, sender) in bridge.pending.lock().await.drain() {
                    let _ = sender.send(Err("zot bridge closed".to_string()));
                }
            });
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            create_session,
            prompt,
            abort,
            close_session,
            export_history,
            import_history,
            extract_openai_account_id
        ])
        .build()
}
