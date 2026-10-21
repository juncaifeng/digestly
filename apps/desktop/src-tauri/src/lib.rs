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

/// 把打包进 resources 的 agent skills 安装到 ~/.agents/skills/<name>/。
/// 幂等: 每次启动覆盖同名文件,保证升级后 skill 同步更新;用户自建目录不受影响。
#[cfg(desktop)]
fn install_skills(app: &tauri::App) {
    let skills_dir = app
        .path()
        .resource_dir()
        .map(|d| d.join("resources").join("skills"));
    let home = app.path().home_dir();
    let (Ok(skills_dir), Ok(home)) = (skills_dir, home) else {
        eprintln!("skills install skipped: cannot resolve resource/home dir");
        return;
    };
    if !skills_dir.is_dir() {
        eprintln!("skills install skipped: {} not found", skills_dir.display());
        return;
    }
    let target_root = home.join(".agents").join("skills");
    let Ok(entries) = std::fs::read_dir(&skills_dir) else { return };
    for entry in entries.flatten() {
        let src = entry.path();
        if !src.is_dir() {
            continue;
        }
        let dst = target_root.join(entry.file_name());
        if let Err(e) = copy_dir(&src, &dst) {
            eprintln!("skill {:?} install failed: {e}", entry.file_name());
        }
    }
}

#[cfg(desktop)]
fn copy_dir(src: &std::path::Path, dst: &std::path::Path) -> std::io::Result<()> {
    std::fs::create_dir_all(dst)?;
    for entry in std::fs::read_dir(src)? {
        let entry = entry?;
        let from = entry.path();
        let to = dst.join(entry.file_name());
        if from.is_dir() {
            copy_dir(&from, &to)?;
        } else {
            std::fs::copy(&from, &to)?;
        }
    }
    Ok(())
}

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
                install_skills(app);
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
