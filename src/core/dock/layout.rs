//! Modelo puro del layout: un árbol de divisiones cuyas hojas son grupos de pestañas.
//! Sin Leptos ni DOM, para poder probarlo y serializarlo (persistencia en el navegador).

use serde::{Deserialize, Serialize};

pub type TabId = u64;
pub type GroupId = u64;
pub type SplitId = u64;

/// Razón mínima/máxima que ocupa el primer hijo de una división.
const MIN_RATIO: f32 = 0.12;
const MAX_RATIO: f32 = 0.88;

/// Una pestaña: la ruta identifica el contenido (se reutiliza si ya está abierta).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct TabEntry {
    pub id: TabId,
    pub path: String,
    pub title: String,
}

/// Grupo de pestañas con una activa.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct TabGroup {
    pub id: GroupId,
    pub tabs: Vec<TabEntry>,
    pub active: Option<TabId>,
}

/// Dirección de una división: `Row` coloca los hijos lado a lado, `Column` uno sobre otro.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SplitDir {
    Row,
    Column,
}

/// Zona de un grupo sobre la que se suelta una pestaña.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DropZone {
    Center,
    Left,
    Right,
    Top,
    Bottom,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum LayoutNode {
    Group(GroupId),
    Split {
        id: SplitId,
        dir: SplitDir,
        /// Fracción (0..1) que ocupa `first`.
        ratio: f32,
        first: Box<LayoutNode>,
        second: Box<LayoutNode>,
    },
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct DockLayout {
    pub root: LayoutNode,
    pub groups: Vec<TabGroup>,
    pub focused: GroupId,
    next_id: u64,
}

impl Default for DockLayout {
    fn default() -> Self {
        Self::new()
    }
}

/// Resultado de quitar una hoja de un árbol: `None` si el árbol quedó vacío.
fn without_group(node: LayoutNode, gid: GroupId) -> Option<LayoutNode> {
    match node {
        LayoutNode::Group(id) if id == gid => None,
        LayoutNode::Group(_) => Some(node),
        LayoutNode::Split {
            id,
            dir,
            ratio,
            first,
            second,
        } => match (without_group(*first, gid), without_group(*second, gid)) {
            (Some(a), Some(b)) => Some(LayoutNode::Split {
                id,
                dir,
                ratio,
                first: Box::new(a),
                second: Box::new(b),
            }),
            (Some(only), None) | (None, Some(only)) => Some(only),
            (None, None) => None,
        },
    }
}

/// Sustituye la hoja `target` por una división con la hoja nueva `new_group`.
fn split_leaf(
    node: LayoutNode,
    target: GroupId,
    new_group: GroupId,
    split_id: SplitId,
    dir: SplitDir,
    new_first: bool,
) -> LayoutNode {
    match node {
        LayoutNode::Group(id) if id == target => {
            let old: Box<LayoutNode> = Box::new(LayoutNode::Group(id));
            let fresh: Box<LayoutNode> = Box::new(LayoutNode::Group(new_group));
            let (first, second) = if new_first {
                (fresh, old)
            } else {
                (old, fresh)
            };
            LayoutNode::Split {
                id: split_id,
                dir,
                ratio: 0.5,
                first,
                second,
            }
        }
        LayoutNode::Group(_) => node,
        LayoutNode::Split {
            id,
            dir: d,
            ratio,
            first,
            second,
        } => LayoutNode::Split {
            id,
            dir: d,
            ratio,
            first: Box::new(split_leaf(
                *first, target, new_group, split_id, dir, new_first,
            )),
            second: Box::new(split_leaf(
                *second, target, new_group, split_id, dir, new_first,
            )),
        },
    }
}

fn set_split_ratio(node: &mut LayoutNode, split_id: SplitId, value: f32) -> bool {
    match node {
        LayoutNode::Group(_) => false,
        LayoutNode::Split {
            id,
            ratio,
            first,
            second,
            ..
        } => {
            if *id == split_id {
                *ratio = value.clamp(MIN_RATIO, MAX_RATIO);
                true
            } else {
                set_split_ratio(first, split_id, value) || set_split_ratio(second, split_id, value)
            }
        }
    }
}

fn collect_leaves(node: &LayoutNode, out: &mut Vec<GroupId>) {
    match node {
        LayoutNode::Group(id) => out.push(*id),
        LayoutNode::Split { first, second, .. } => {
            collect_leaves(first, out);
            collect_leaves(second, out);
        }
    }
}

impl LayoutNode {
    /// Copia del árbol con todas las razones a cero: dos layouts con la misma forma son
    /// iguales aunque se haya arrastrado un divisor (así redimensionar no reconstruye la UI).
    pub fn shape(&self) -> LayoutNode {
        match self {
            LayoutNode::Group(id) => LayoutNode::Group(*id),
            LayoutNode::Split {
                id,
                dir,
                first,
                second,
                ..
            } => LayoutNode::Split {
                id: *id,
                dir: *dir,
                ratio: 0.0,
                first: Box::new(first.shape()),
                second: Box::new(second.shape()),
            },
        }
    }

    /// Razón actual de la división `split`.
    pub fn ratio_of(&self, split: SplitId) -> Option<f32> {
        match self {
            LayoutNode::Group(_) => None,
            LayoutNode::Split {
                id,
                ratio,
                first,
                second,
                ..
            } => {
                if *id == split {
                    Some(*ratio)
                } else {
                    first.ratio_of(split).or_else(|| second.ratio_of(split))
                }
            }
        }
    }
}

impl DockLayout {
    /// Layout inicial: un único grupo vacío.
    pub fn new() -> Self {
        Self {
            root: LayoutNode::Group(1),
            groups: vec![TabGroup {
                id: 1,
                tabs: Vec::new(),
                active: None,
            }],
            focused: 1,
            next_id: 2,
        }
    }

    fn fresh_id(&mut self) -> u64 {
        let id: u64 = self.next_id;
        self.next_id += 1;
        id
    }

    pub fn group(&self, id: GroupId) -> Option<&TabGroup> {
        self.groups.iter().find(|g: &&TabGroup| g.id == id)
    }

    fn group_mut(&mut self, id: GroupId) -> Option<&mut TabGroup> {
        self.groups.iter_mut().find(|g: &&mut TabGroup| g.id == id)
    }

    /// Grupo que contiene la pestaña.
    pub fn group_of_tab(&self, tab: TabId) -> Option<GroupId> {
        self.groups
            .iter()
            .find(|g: &&TabGroup| g.tabs.iter().any(|t: &TabEntry| t.id == tab))
            .map(|g: &TabGroup| g.id)
    }

    /// Pestaña (y su grupo) que muestra `path`, si está abierta.
    pub fn find_path(&self, path: &str) -> Option<(GroupId, TabId)> {
        self.groups.iter().find_map(|g: &TabGroup| {
            g.tabs
                .iter()
                .find(|t: &&TabEntry| t.path == path)
                .map(|t: &TabEntry| (g.id, t.id))
        })
    }

    /// Grupos en orden de aparición en el árbol.
    pub fn leaf_order(&self) -> Vec<GroupId> {
        let mut out: Vec<GroupId> = Vec::new();
        collect_leaves(&self.root, &mut out);
        out
    }

    /// Rutas de la pestaña activa de cada grupo (lo que el usuario está viendo).
    pub fn visible_paths(&self) -> Vec<String> {
        self.groups
            .iter()
            .filter_map(|g: &TabGroup| {
                let active: TabId = g.active?;
                g.tabs
                    .iter()
                    .find(|t: &&TabEntry| t.id == active)
                    .map(|t: &TabEntry| t.path.clone())
            })
            .collect()
    }

    /// Ruta de la pestaña activa del grupo con foco.
    pub fn focused_path(&self) -> Option<String> {
        let group: &TabGroup = self.group(self.focused)?;
        let active: TabId = group.active?;
        group
            .tabs
            .iter()
            .find(|t: &&TabEntry| t.id == active)
            .map(|t: &TabEntry| t.path.clone())
    }

    pub fn is_path_visible(&self, path: &str) -> bool {
        self.visible_paths().iter().any(|p: &String| p == path)
    }

    pub fn focus(&mut self, group: GroupId) {
        if self.group(group).is_some() {
            self.focused = group;
        }
    }

    pub fn activate(&mut self, tab: TabId) {
        if let Some(gid) = self.group_of_tab(tab) {
            if let Some(g) = self.group_mut(gid) {
                g.active = Some(tab);
            }
            self.focused = gid;
        }
    }

    /// Abre `path`: si ya hay una pestaña con esa ruta la activa; si no, crea una en `group`
    /// (o en el grupo con foco) y la activa.
    pub fn open(&mut self, path: &str, title: &str, group: Option<GroupId>) -> TabId {
        if let Some((_, tab)) = self.find_path(path) {
            self.activate(tab);
            return tab;
        }
        let target: GroupId = group
            .filter(|g: &GroupId| self.group(*g).is_some())
            .unwrap_or(self.focused);
        let id: TabId = self.fresh_id();
        if let Some(g) = self.group_mut(target) {
            g.tabs.push(TabEntry {
                id,
                path: path.to_string(),
                title: title.to_string(),
            });
            g.active = Some(id);
        }
        self.focused = target;
        id
    }

    /// Cambia la ruta y el título de una pestaña existente (navegación dentro de la pestaña).
    pub fn replace_path(&mut self, tab: TabId, path: &str, title: &str) {
        if let Some((_, existing)) = self.find_path(path)
            && existing != tab
        {
            self.close_tab(tab);
            self.activate(existing);
            return;
        }
        for g in self.groups.iter_mut() {
            if let Some(t) = g.tabs.iter_mut().find(|t: &&mut TabEntry| t.id == tab) {
                t.path = path.to_string();
                t.title = title.to_string();
            }
        }
    }

    /// Actualiza el título de la pestaña que muestra `path`.
    pub fn set_title(&mut self, path: &str, title: &str) {
        for g in self.groups.iter_mut() {
            for t in g.tabs.iter_mut().filter(|t: &&mut TabEntry| t.path == path) {
                if t.title != title {
                    t.title = title.to_string();
                }
            }
        }
    }

    fn remove_group(&mut self, gid: GroupId) {
        if self.groups.len() <= 1 {
            return;
        }
        let order: Vec<GroupId> = self.leaf_order();
        let position: usize = order.iter().position(|g: &GroupId| *g == gid).unwrap_or(0);
        let root: LayoutNode = self.root.clone();
        if let Some(next) = without_group(root, gid) {
            self.root = next;
        }
        self.groups.retain(|g: &TabGroup| g.id != gid);
        if self.focused == gid {
            let remaining: Vec<GroupId> = self.leaf_order();
            self.focused = remaining
                .get(position.saturating_sub(1))
                .or_else(|| remaining.first())
                .copied()
                .unwrap_or(1);
        }
    }

    /// Cierra una pestaña; un grupo que queda vacío desaparece salvo que sea el último.
    pub fn close_tab(&mut self, tab: TabId) {
        let Some(gid) = self.group_of_tab(tab) else {
            return;
        };
        let mut emptied: bool = false;
        if let Some(g) = self.group_mut(gid) {
            let index: usize = g
                .tabs
                .iter()
                .position(|t: &TabEntry| t.id == tab)
                .unwrap_or(0);
            g.tabs.retain(|t: &TabEntry| t.id != tab);
            if g.active == Some(tab) {
                g.active = g
                    .tabs
                    .get(index)
                    .or_else(|| g.tabs.last())
                    .map(|t: &TabEntry| t.id);
            }
            emptied = g.tabs.is_empty();
        }
        if emptied {
            self.remove_group(gid);
        }
    }

    /// Cierra la pestaña que muestra `path`, si existe.
    pub fn close_path(&mut self, path: &str) {
        if let Some((_, tab)) = self.find_path(path) {
            self.close_tab(tab);
        }
    }

    /// Cierra todas las demás pestañas del grupo de `tab`.
    pub fn close_others(&mut self, tab: TabId) {
        let Some(gid) = self.group_of_tab(tab) else {
            return;
        };
        let others: Vec<TabId> = self
            .group(gid)
            .map(|g: &TabGroup| {
                g.tabs
                    .iter()
                    .filter(|t: &&TabEntry| t.id != tab)
                    .map(|t: &TabEntry| t.id)
                    .collect()
            })
            .unwrap_or_default();
        for other in others {
            self.close_tab(other);
        }
    }

    /// Divide `group` creando uno nuevo y vacío a su derecha (`Row`) o debajo (`Column`).
    pub fn split_group(&mut self, group: GroupId, dir: SplitDir) -> GroupId {
        let new_group: GroupId = self.fresh_id();
        let split_id: SplitId = self.fresh_id();
        let root: LayoutNode = self.root.clone();
        self.root = split_leaf(root, group, new_group, split_id, dir, false);
        self.groups.push(TabGroup {
            id: new_group,
            tabs: Vec::new(),
            active: None,
        });
        self.focused = new_group;
        new_group
    }

    /// Mueve una pestaña a otro grupo (`Center`) o a una división nueva en el borde indicado.
    pub fn move_tab(&mut self, tab: TabId, target: GroupId, zone: DropZone) {
        let Some(source) = self.group_of_tab(tab) else {
            return;
        };
        if self.group(target).is_none() {
            return;
        }
        let source_len: usize = self.group(source).map_or(0, |g: &TabGroup| g.tabs.len());
        if source == target && (zone == DropZone::Center || source_len <= 1) {
            self.activate(tab);
            return;
        }
        let entry: Option<TabEntry> = self.group_mut(source).and_then(|g: &mut TabGroup| {
            let index: usize = g.tabs.iter().position(|t: &TabEntry| t.id == tab)?;
            let entry: TabEntry = g.tabs.remove(index);
            if g.active == Some(tab) {
                g.active = g
                    .tabs
                    .get(index)
                    .or_else(|| g.tabs.last())
                    .map(|t: &TabEntry| t.id);
            }
            Some(entry)
        });
        let Some(entry) = entry else {
            return;
        };
        let destination: GroupId = match zone {
            DropZone::Center => target,
            edge => {
                let new_group: GroupId = self.fresh_id();
                let split_id: SplitId = self.fresh_id();
                let (dir, new_first): (SplitDir, bool) = match edge {
                    DropZone::Left => (SplitDir::Row, true),
                    DropZone::Right => (SplitDir::Row, false),
                    DropZone::Top => (SplitDir::Column, true),
                    _ => (SplitDir::Column, false),
                };
                let root: LayoutNode = self.root.clone();
                self.root = split_leaf(root, target, new_group, split_id, dir, new_first);
                self.groups.push(TabGroup {
                    id: new_group,
                    tabs: Vec::new(),
                    active: None,
                });
                new_group
            }
        };
        if let Some(g) = self.group_mut(destination) {
            g.active = Some(entry.id);
            g.tabs.push(entry);
        }
        self.focused = destination;
        if self
            .group(source)
            .is_some_and(|g: &TabGroup| g.tabs.is_empty())
        {
            self.remove_group(source);
        }
    }

    /// Mueve la pestaña a una división nueva junto a su propio grupo.
    pub fn move_to_new_split(&mut self, tab: TabId, dir: SplitDir) {
        if let Some(gid) = self.group_of_tab(tab) {
            let zone: DropZone = match dir {
                SplitDir::Row => DropZone::Right,
                SplitDir::Column => DropZone::Bottom,
            };
            self.move_tab(tab, gid, zone);
        }
    }

    pub fn set_ratio(&mut self, split: SplitId, ratio: f32) {
        set_split_ratio(&mut self.root, split, ratio);
    }

    /// Comprueba que el árbol y los grupos son coherentes (datos del navegador no fiables).
    pub fn is_consistent(&self) -> bool {
        let leaves: Vec<GroupId> = self.leaf_order();
        let mut sorted: Vec<GroupId> = leaves.clone();
        sorted.sort_unstable();
        sorted.dedup();
        sorted.len() == leaves.len()
            && leaves.len() == self.groups.len()
            && leaves.iter().all(|id: &GroupId| self.group(*id).is_some())
            && self.group(self.focused).is_some()
            && self.groups.iter().all(|g: &TabGroup| {
                g.active
                    .is_none_or(|a: TabId| g.tabs.iter().any(|t: &TabEntry| t.id == a))
            })
    }

    pub fn to_json(&self) -> String {
        serde_json::to_string(self).unwrap_or_default()
    }

    /// Restaura un layout guardado; devuelve `None` si el JSON no es válido o incoherente.
    pub fn from_json(json: &str) -> Option<Self> {
        let layout: Self = serde_json::from_str(json).ok()?;
        layout.is_consistent().then_some(layout)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn two_tabs() -> (DockLayout, TabId, TabId) {
        let mut layout: DockLayout = DockLayout::new();
        let a: TabId = layout.open("/a", "A", None);
        let b: TabId = layout.open("/b", "B", None);
        (layout, a, b)
    }

    #[test]
    fn open_adds_and_activates_the_tab() {
        let (layout, _, b) = two_tabs();
        let group: &TabGroup = layout.group(layout.focused).expect("focused group");
        assert_eq!(group.tabs.len(), 2);
        assert_eq!(group.active, Some(b));
    }

    #[test]
    fn open_existing_path_activates_instead_of_duplicating() {
        let (mut layout, a, _) = two_tabs();
        let again: TabId = layout.open("/a", "A", None);
        assert_eq!(again, a);
        assert_eq!(layout.groups[0].tabs.len(), 2);
        assert_eq!(layout.groups[0].active, Some(a));
    }

    #[test]
    fn close_active_tab_selects_neighbour_and_last_group_survives() {
        let (mut layout, a, b) = two_tabs();
        layout.close_tab(b);
        assert_eq!(layout.groups[0].active, Some(a));
        layout.close_tab(a);
        assert_eq!(layout.groups.len(), 1);
        assert!(layout.groups[0].tabs.is_empty());
        assert_eq!(layout.groups[0].active, None);
    }

    #[test]
    fn split_group_adds_an_empty_focused_group() {
        let (mut layout, _, _) = two_tabs();
        let new_group: GroupId = layout.split_group(1, SplitDir::Row);
        assert_eq!(layout.groups.len(), 2);
        assert_eq!(layout.focused, new_group);
        assert_eq!(layout.leaf_order(), vec![1, new_group]);
        assert!(layout.is_consistent());
    }

    #[test]
    fn closing_last_tab_of_a_split_group_collapses_the_split() {
        let (mut layout, _, _) = two_tabs();
        let new_group: GroupId = layout.split_group(1, SplitDir::Row);
        let tab: TabId = layout.open("/c", "C", Some(new_group));
        layout.close_tab(tab);
        assert_eq!(layout.groups.len(), 1);
        assert_eq!(layout.root, LayoutNode::Group(1));
        assert_eq!(layout.focused, 1);
    }

    #[test]
    fn move_tab_to_edge_creates_a_new_split_on_that_side() {
        let (mut layout, a, _) = two_tabs();
        layout.move_tab(a, 1, DropZone::Left);
        assert_eq!(layout.groups.len(), 2);
        let order: Vec<GroupId> = layout.leaf_order();
        assert_eq!(order[1], 1, "the original group stays on the right");
        assert_eq!(layout.group_of_tab(a), Some(order[0]));
        assert!(layout.is_consistent());
    }

    #[test]
    fn move_tab_to_center_of_another_group_and_remove_emptied_source() {
        let (mut layout, a, b) = two_tabs();
        layout.move_tab(b, 1, DropZone::Bottom);
        let bottom: GroupId = layout.group_of_tab(b).expect("b group");
        layout.move_tab(b, 1, DropZone::Center);
        assert_eq!(layout.groups.len(), 1, "emptied group is removed: {bottom}");
        assert_eq!(layout.group_of_tab(a), Some(1));
        assert_eq!(layout.group_of_tab(b), Some(1));
        assert_eq!(layout.root, LayoutNode::Group(1));
    }

    #[test]
    fn moving_the_only_tab_onto_its_own_edge_is_a_noop() {
        let mut layout: DockLayout = DockLayout::new();
        let a: TabId = layout.open("/a", "A", None);
        layout.move_tab(a, 1, DropZone::Right);
        assert_eq!(layout.groups.len(), 1);
    }

    #[test]
    fn set_ratio_is_clamped() {
        let (mut layout, _, _) = two_tabs();
        layout.split_group(1, SplitDir::Column);
        let LayoutNode::Split { id, .. } = layout.root.clone() else {
            panic!("expected a split");
        };
        layout.set_ratio(id, 0.0);
        let LayoutNode::Split { ratio, .. } = layout.root else {
            panic!("expected a split");
        };
        assert!((ratio - MIN_RATIO).abs() < f32::EPSILON);
    }

    #[test]
    fn shape_ignores_ratios_but_ratio_of_reads_them() {
        let (mut layout, _, _) = two_tabs();
        layout.split_group(1, SplitDir::Row);
        let before: LayoutNode = layout.root.shape();
        let LayoutNode::Split { id, .. } = layout.root.clone() else {
            panic!("expected a split");
        };
        layout.set_ratio(id, 0.3);
        assert_eq!(layout.root.shape(), before);
        assert_eq!(layout.root.ratio_of(id), Some(0.3));
        assert_eq!(layout.root.ratio_of(999), None);
    }

    #[test]
    fn replace_path_merges_into_an_existing_tab_for_that_path() {
        let (mut layout, a, b) = two_tabs();
        layout.replace_path(b, "/a", "A");
        assert_eq!(layout.groups[0].tabs.len(), 1);
        assert_eq!(layout.groups[0].active, Some(a));
    }

    #[test]
    fn json_round_trip_and_rejects_inconsistent_data() {
        let (mut layout, _, _) = two_tabs();
        layout.split_group(1, SplitDir::Row);
        let restored: Option<DockLayout> = DockLayout::from_json(&layout.to_json());
        assert_eq!(restored, Some(layout.clone()));
        let mut broken: DockLayout = layout;
        broken.groups.pop();
        assert_eq!(DockLayout::from_json(&broken.to_json()), None);
        assert_eq!(DockLayout::from_json("not json"), None);
    }

    #[test]
    fn focused_path_follows_the_focused_group() {
        let (mut layout, _, _) = two_tabs();
        assert_eq!(layout.focused_path().as_deref(), Some("/b"));
        let g: GroupId = layout.split_group(1, SplitDir::Row);
        assert_eq!(layout.focused_path(), None, "new group is empty");
        layout.open("/c", "C", Some(g));
        assert_eq!(layout.focused_path().as_deref(), Some("/c"));
        layout.focus(1);
        assert_eq!(layout.focused_path().as_deref(), Some("/b"));
    }

    #[test]
    fn visible_paths_lists_the_active_tab_of_every_group() {
        let (mut layout, _, _) = two_tabs();
        let g: GroupId = layout.split_group(1, SplitDir::Row);
        layout.open("/c", "C", Some(g));
        let mut paths: Vec<String> = layout.visible_paths();
        paths.sort();
        assert_eq!(paths, vec!["/b".to_string(), "/c".to_string()]);
        assert!(layout.is_path_visible("/c"));
        assert!(!layout.is_path_visible("/a"));
    }
}
