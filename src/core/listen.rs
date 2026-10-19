//! Address the server binds to: loopback by default, every interface when the user opts in.

use std::net::{IpAddr, Ipv4Addr, SocketAddr, UdpSocket};

/// Environment variable that pins the bind address (also read by Leptos).
pub const SITE_ADDR_VAR: &str = "LEPTOS_SITE_ADDR";
/// Probe address used only to ask the OS which interface it would route through; nothing is sent.
const ROUTE_PROBE: (Ipv4Addr, u16) = (Ipv4Addr::new(192, 0, 2, 1), 9);

/// Where the running server listens and whether the environment (not the user setting) decided it.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ListenInfo {
    pub addr: SocketAddr,
    /// `LEPTOS_SITE_ADDR` was set, so the saved preference is ignored.
    pub locked_by_env: bool,
    /// The running server demands the access token (only meaningful while exposed).
    pub token_required: bool,
}

impl ListenInfo {
    /// `true` when the server accepts connections from other machines.
    pub fn is_exposed(&self) -> bool {
        !self.addr.ip().is_loopback()
    }
}

/// Chooses the bind address. An explicit environment address always wins; otherwise opting in to
/// network access listens on every IPv4 interface keeping the configured port, and the default
/// stays on the configured (loopback) address.
pub fn resolve_listen_addr(
    configured: SocketAddr,
    env_locked: bool,
    network_access: bool,
) -> SocketAddr {
    if env_locked || !network_access {
        return configured;
    }
    SocketAddr::new(IpAddr::V4(Ipv4Addr::UNSPECIFIED), configured.port())
}

/// A network interface of this machine that other devices could reach the server through.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NetworkInterface {
    pub name: String,
    pub ip: Ipv4Addr,
}

/// Address of this machine on the local network, if it has one.
pub fn local_network_ip() -> Option<IpAddr> {
    let socket: UdpSocket = UdpSocket::bind((Ipv4Addr::UNSPECIFIED, 0)).ok()?;
    socket.connect(ROUTE_PROBE).ok()?;
    let ip: IpAddr = socket.local_addr().ok()?.ip();
    (!ip.is_loopback() && !ip.is_unspecified()).then_some(ip)
}

/// Keeps the addresses a phone could use: up, not loopback, unspecified or link-local, and
/// listed once. `candidates` are `(interface name, address, is up)`.
pub fn reachable_interfaces(candidates: Vec<(String, Ipv4Addr, bool)>) -> Vec<NetworkInterface> {
    let mut interfaces: Vec<NetworkInterface> = Vec::new();
    for (name, ip, up) in candidates {
        let usable: bool = up && !ip.is_loopback() && !ip.is_unspecified() && !ip.is_link_local();
        let seen: bool = interfaces
            .iter()
            .any(|known: &NetworkInterface| known.ip == ip);
        if usable && !seen {
            interfaces.push(NetworkInterface { name, ip });
        }
    }
    interfaces
}

/// IPv4 interfaces of this machine (ethernet, Wi-Fi, Tailscale…), in the order the OS lists them.
#[cfg(unix)]
pub fn local_network_interfaces() -> Vec<NetworkInterface> {
    use nix::ifaddrs::{InterfaceAddress, getifaddrs};
    use nix::net::if_::InterfaceFlags;

    let Ok(addresses) = getifaddrs() else {
        return Vec::new();
    };
    let candidates: Vec<(String, Ipv4Addr, bool)> = addresses
        .filter_map(|entry: InterfaceAddress| {
            let ip: Ipv4Addr = entry.address?.as_sockaddr_in()?.ip();
            let up: bool = entry.flags.contains(InterfaceFlags::IFF_UP);
            Some((entry.interface_name, ip, up))
        })
        .collect();
    reachable_interfaces(candidates)
}

/// Without interface enumeration, offer only the address the OS routes through.
#[cfg(not(unix))]
pub fn local_network_interfaces() -> Vec<NetworkInterface> {
    match local_network_ip() {
        Some(IpAddr::V4(ip)) => reachable_interfaces(vec![("default".to_string(), ip, true)]),
        _ => Vec::new(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn loopback() -> SocketAddr {
        "127.0.0.1:3000".parse().expect("addr")
    }

    #[test]
    fn stays_on_loopback_by_default() {
        assert_eq!(resolve_listen_addr(loopback(), false, false), loopback());
    }

    #[test]
    fn opting_in_binds_every_interface_keeping_the_port() {
        let expected: SocketAddr = "0.0.0.0:3000".parse().expect("addr");
        assert_eq!(resolve_listen_addr(loopback(), false, true), expected);
    }

    #[test]
    fn environment_address_wins_over_the_preference() {
        let pinned: SocketAddr = "127.0.0.1:9100".parse().expect("addr");
        assert_eq!(resolve_listen_addr(pinned, true, true), pinned);
    }

    fn candidate(name: &str, ip: [u8; 4], up: bool) -> (String, Ipv4Addr, bool) {
        (name.to_string(), Ipv4Addr::from(ip), up)
    }

    #[test]
    fn keeps_ethernet_and_tailscale_addresses() {
        let found: Vec<NetworkInterface> = reachable_interfaces(vec![
            candidate("eth0", [192, 168, 1, 5], true),
            candidate("tailscale0", [100, 64, 0, 7], true),
        ]);
        let names: Vec<&str> = found
            .iter()
            .map(|i: &NetworkInterface| i.name.as_str())
            .collect();
        assert_eq!(names, vec!["eth0", "tailscale0"]);
    }

    #[test]
    fn drops_loopback_link_local_down_and_duplicate_addresses() {
        let found: Vec<NetworkInterface> = reachable_interfaces(vec![
            candidate("lo", [127, 0, 0, 1], true),
            candidate("wlan0", [169, 254, 3, 3], true),
            candidate("eth1", [10, 0, 0, 2], false),
            candidate("eth0", [192, 168, 1, 5], true),
            candidate("eth0:1", [192, 168, 1, 5], true),
        ]);
        assert_eq!(found.len(), 1);
        assert_eq!(found[0].name, "eth0");
    }

    #[test]
    fn exposure_follows_the_bound_ip() {
        let exposed: ListenInfo = ListenInfo {
            addr: "0.0.0.0:3000".parse().expect("addr"),
            locked_by_env: false,
            token_required: true,
        };
        let local: ListenInfo = ListenInfo {
            addr: loopback(),
            locked_by_env: false,
            token_required: true,
        };
        assert!(exposed.is_exposed());
        assert!(!local.is_exposed());
    }
}
