use leptos::prelude::*;

use crate::modules::preferences::infrastructure::adapters::inbound::dto::network_access_dto::NetworkAccessDto;
use crate::modules::preferences::infrastructure::adapters::inbound::server_fns::{
    RegenerateAccessToken, RestartServer, SaveListenPort, SaveNetworkAccess,
    SaveRequireAccessToken, get_network_access,
};

/// State of the network access card: current exposure plus the actions that change it.
#[derive(Clone, Copy)]
pub struct NetworkAccessState {
    pub access: Resource<Result<NetworkAccessDto, ServerFnError>>,
    pub save: ServerAction<SaveNetworkAccess>,
    pub save_require_token: ServerAction<SaveRequireAccessToken>,
    pub save_port: ServerAction<SaveListenPort>,
    pub regenerate: ServerAction<RegenerateAccessToken>,
    /// Restarts the server so the saved settings apply.
    pub restart: ServerAction<RestartServer>,
    /// Address of the interface whose QR code is shown; `None` means the preferred one.
    pub selected_address: RwSignal<Option<String>>,
}

impl NetworkAccessState {
    pub fn new() -> Self {
        let save: ServerAction<SaveNetworkAccess> = ServerAction::new();
        let save_require_token: ServerAction<SaveRequireAccessToken> = ServerAction::new();
        let save_port: ServerAction<SaveListenPort> = ServerAction::new();
        let regenerate: ServerAction<RegenerateAccessToken> = ServerAction::new();
        let restart: ServerAction<RestartServer> = ServerAction::new();
        let access: Resource<Result<NetworkAccessDto, ServerFnError>> = Resource::new(
            move || {
                (
                    save.version().get(),
                    save_require_token.version().get(),
                    save_port.version().get(),
                    regenerate.version().get(),
                )
            },
            move |_| get_network_access(),
        );
        Self {
            access,
            save,
            save_require_token,
            save_port,
            regenerate,
            restart,
            selected_address: RwSignal::new(None),
        }
    }

    /// Any of the actions is still running.
    pub fn busy(&self) -> Signal<bool> {
        let (save, require, port, regenerate, restart) = (
            self.save,
            self.save_require_token,
            self.save_port,
            self.regenerate,
            self.restart,
        );
        Signal::derive(move || {
            save.pending().get()
                || require.pending().get()
                || port.pending().get()
                || regenerate.pending().get()
                || restart.pending().get()
        })
    }
}

impl Default for NetworkAccessState {
    fn default() -> Self {
        Self::new()
    }
}
