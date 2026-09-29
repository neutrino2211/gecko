use zed_extension_api as zed;

fn resolve_command(
    root: &str,
    compiler_project: bool,
    on_path: Option<String>,
    binary: Option<zed::settings::CommandSettings>,
) -> zed::Result<zed::Command> {
    let command = binary
        .as_ref()
        .and_then(|settings| settings.path.as_ref())
        .filter(|path| !path.is_empty())
        .cloned()
        .or_else(|| compiler_project.then(|| format!("{root}/gecko-lsp")))
        .or(on_path)
        .ok_or_else(|| {
            "gecko-lsp was not found; set lsp.gecko-lsp.binary.path in Zed settings or put it on PATH"
                .to_string()
        })?;
    let args = binary
        .as_ref()
        .and_then(|settings| settings.arguments.clone())
        .unwrap_or_default();
    let mut env = binary.and_then(|settings| settings.env).unwrap_or_default();
    if compiler_project {
        env.entry("GECKO_HOME".to_string())
            .or_insert_with(|| root.to_string());
    }
    Ok(zed::Command {
        command,
        args,
        env: env.into_iter().collect(),
    })
}

struct GeckoExtension;

impl zed::Extension for GeckoExtension {
    fn new() -> Self {
        Self
    }

    fn language_server_command(
        &mut self,
        language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<zed::Command> {
        let root = worktree.root_path();
        let compiler_project = worktree
            .read_text_file("go.mod")
            .is_ok_and(|content| content.contains("module github.com/neutrino2211/gecko"));
        let settings =
            zed::settings::LspSettings::for_worktree(language_server_id.as_ref(), worktree)?;
        resolve_command(
            &root,
            compiler_project,
            worktree.which("gecko-lsp"),
            settings.binary,
        )
    }
}

zed::register_extension!(GeckoExtension);

#[cfg(test)]
mod tests {
    use super::resolve_command;
    use std::collections::HashMap;
    use zed_extension_api::settings::CommandSettings;

    #[test]
    fn configured_binary_takes_precedence() {
        let binary = CommandSettings {
            path: Some("/opt/gecko/gecko-lsp".to_string()),
            arguments: Some(vec!["--stdio".to_string()]),
            env: Some(HashMap::from([(
                "GECKO_HOME".to_string(),
                "/opt/gecko".to_string(),
            )])),
        };
        let command = resolve_command("/project", false, None, Some(binary)).unwrap();
        assert_eq!(command.command, "/opt/gecko/gecko-lsp");
        assert_eq!(command.args, vec!["--stdio"]);
        assert_eq!(
            command.env,
            vec![("GECKO_HOME".to_string(), "/opt/gecko".to_string())]
        );
    }

    #[test]
    fn compiler_project_uses_its_own_binary() {
        let command =
            resolve_command("/gecko", true, Some("/bin/gecko-lsp".to_string()), None).unwrap();
        assert_eq!(command.command, "/gecko/gecko-lsp");
        assert_eq!(
            command.env,
            vec![("GECKO_HOME".to_string(), "/gecko".to_string())]
        );
    }

    #[test]
    fn external_project_uses_path() {
        let command =
            resolve_command("/project", false, Some("/bin/gecko-lsp".to_string()), None).unwrap();
        assert_eq!(command.command, "/bin/gecko-lsp");
    }

    #[test]
    fn missing_binary_explains_configuration() {
        let error = resolve_command("/project", false, None, None).unwrap_err();
        assert!(error.contains("lsp.gecko-lsp.binary.path"));
    }
}
