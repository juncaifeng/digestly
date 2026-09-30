//! digestly 壳:桌面端启动 Go sidecar(digestly-server)并编排其生命周期。
//! 移动端(Android/iOS)不支持子进程 sidecar,后续由 gomobile 本地库替代。
//! 业务逻辑全部在 Go core,这里只做进程编排。

#[cfg(desktop)]
use tauri::Manager;
#[cfg(desktop)]
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
#[cfg(desktop)]
use tauri_plugin_shell::ShellExt;

#[cfg(desktop)]
struct SidecarState(std::sync::Mutex<Option<CommandChild>>);

#[cfg(desktop)]
fn spawn_sidecar(app: &tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    let sidecar = app.shell().sidecar("digestly-server")?;
    let data_dir = app
        .path()
        .app_data_dir()
        .unwrap_or_else(|_| std::path::PathBuf::from("data"));
    std::fs::create_dir_all(&data_dir).ok();
    let (mut rx, child) = sidecar
        .args(["-data", &data_dir.to_string_lossy()])
        .spawn()?;
    app.manage(SidecarState(std::sync::Mutex::new(Some(child))));
    tauri::async_runtime::spawn(async move {
        while let Some(event) = rx.recv().await {
            if let CommandEvent::Stdout(line) | CommandEvent::Stderr(line) = event {
                println!("[server] {}", String::from_utf8_lossy(&line));
            }
        }
    });
    Ok(())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    #[allow(unused_mut)]
    let mut builder = tauri::Builder::default();

    #[cfg(desktop)]
    {
        builder = builder
            .plugin(tauri_plugin_shell::init())
            .setup(|app| {
                if let Err(e) = spawn_sidecar(app) {
                    eprintln!("sidecar spawn failed: {e}");
                }
                Ok(())
            })
            .on_window_event(|window, event| {
                if let tauri::WindowEvent::Destroyed = event {
                    // 窗口关闭时杀掉 sidecar,避免孤儿进程
                    if let Some(state) = window.try_state::<SidecarState>() {
                        if let Some(child) = state.0.lock().unwrap().take() {
                            let _ = child.kill();
                        }
                    }
                }
            });
    }

    builder
        .run(tauri::generate_context!())
        .expect("error while running digestly");
}
