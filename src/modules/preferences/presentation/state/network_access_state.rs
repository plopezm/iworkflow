use leptos::prelude::*;

use crate::modules::preferences::infrastructure::adapters::inbound::dto::network_access_dto::NetworkAccessDto;
use crate::modules::preferences::infrastructure::adapters::inbound::server_fns::{
    RegenerateAccessToken, SaveNetworkAccess, SaveRequireAccessToken, get_network_access,
};

/// State of the network access card: current exposure plus the actions that change it.
#[derive(Clone, Copy)]
pub struct NetworkAccessState {
    pub access: Resource<Result<NetworkAccessDto, ServerFnError>>,
    pub save: ServerAction<SaveNetworkAccess>,
    pub save_require_token: ServerAction<SaveRequireAccessToken>,
    pub regenerate: ServerAction<RegenerateAccessToken>,
}

impl NetworkAccessState {
    pub fn new() -> Self {
        let save: ServerAction<SaveNetworkAccess> = ServerAction::new();
        let save_require_token: ServerAction<SaveRequireAccessToken> = ServerAction::new();
        let regenerate: ServerAction<RegenerateAccessToken> = ServerAction::new();
        let access: Resource<Result<NetworkAccessDto, ServerFnError>> = Resource::new(
            move || {
                (
                    save.version().get(),
                    save_require_token.version().get(),
                    regenerate.version().get(),
                )
            },
            move |_| get_network_access(),
        );
        Self {
            access,
            save,
            save_require_token,
            regenerate,
        }
    }

    /// Any of the three actions is still running.
    pub fn busy(&self) -> Signal<bool> {
        let (save, require, regenerate) = (self.save, self.save_require_token, self.regenerate);
        Signal::derive(move || {
            save.pending().get() || require.pending().get() || regenerate.pending().get()
        })
    }
}

impl Default for NetworkAccessState {
    fn default() -> Self {
        Self::new()
    }
}
