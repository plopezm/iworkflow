use std::collections::BTreeMap;

use serde_json::{Map, Value};

use crate::modules::agent_profiles::domain::models::profile::Profile;
use crate::modules::agent_profiles::domain::models::routing_entry::RoutingEntry;

fn optional_string(object: &Map<String, Value>, key: &str) -> Result<Option<String>, String> {
    match object.get(key) {
        None | Some(Value::Null) => Ok(None),
        Some(Value::String(value)) => Ok(Some(value.clone())),
        Some(_) => Err(format!("`{key}` debe ser texto")),
    }
}

/// Entrada `{model?, thinking?}` o cadena legada `"provider/model"` (solo modelo).
pub fn parse_entry(value: &Value) -> Result<RoutingEntry, String> {
    let entry: RoutingEntry = match value {
        Value::String(model) => RoutingEntry {
            model: Some(model.clone()),
            thinking: None,
        },
        Value::Object(object) => RoutingEntry {
            model: optional_string(object, "model")?,
            thinking: optional_string(object, "thinking")?,
        },
        _ => return Err("se esperaba un objeto o una cadena".to_string()),
    };
    entry.validate().map_err(|e| e.to_string())?;
    Ok(entry)
}

/// Mapa agente -> entrada. Cada error nombra el agente afectado.
pub fn parse_routing(map: &Map<String, Value>) -> Result<BTreeMap<String, RoutingEntry>, String> {
    let mut routing: BTreeMap<String, RoutingEntry> = BTreeMap::new();
    for (agent, value) in map {
        let entry: RoutingEntry =
            parse_entry(value).map_err(|e: String| format!("agente `{agent}`: {e}"))?;
        routing.insert(agent.clone(), entry);
    }
    Profile::validate_routing(&routing).map_err(|e| e.to_string())?;
    Ok(routing)
}

pub fn entry_to_value(entry: &RoutingEntry) -> Value {
    let mut object: Map<String, Value> = Map::new();
    if let Some(model) = &entry.model {
        object.insert("model".to_string(), Value::String(model.clone()));
    }
    if let Some(thinking) = &entry.thinking {
        object.insert("thinking".to_string(), Value::String(thinking.clone()));
    }
    Value::Object(object)
}

/// Claves ordenadas (el origen es un `BTreeMap`).
pub fn routing_to_value(routing: &BTreeMap<String, RoutingEntry>) -> Value {
    let mut object: Map<String, Value> = Map::new();
    for (agent, entry) in routing {
        object.insert(agent.clone(), entry_to_value(entry));
    }
    Value::Object(object)
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;

    #[test]
    fn legacy_string_is_model_only() {
        let entry: RoutingEntry = parse_entry(&json!("p/m")).expect("entry");
        assert_eq!(entry.model.as_deref(), Some("p/m"));
        assert_eq!(entry.thinking, None);
    }

    #[test]
    fn object_with_only_thinking_is_accepted() {
        let entry: RoutingEntry = parse_entry(&json!({"thinking": "low"})).expect("entry");
        assert_eq!(entry.model, None);
        assert_eq!(entry.thinking.as_deref(), Some("low"));
    }

    #[test]
    fn rejects_bad_types_and_values() {
        assert!(parse_entry(&json!(3)).is_err());
        assert!(parse_entry(&json!({"model": 3})).is_err());
        assert!(parse_entry(&json!({"thinking": "ultra"})).is_err());
        assert!(parse_entry(&json!("  ")).is_err());
    }

    #[test]
    fn routing_error_names_agent() {
        let map: Value = json!({"sdd-apply": {"thinking": "ultra"}});
        let Value::Object(object) = map else {
            return;
        };
        let err: String = parse_routing(&object).expect_err("invalid");
        assert!(err.contains("sdd-apply"), "{err}");
    }

    #[test]
    fn serializes_only_present_fields() {
        let entry: RoutingEntry = RoutingEntry {
            model: Some("a/b".to_string()),
            thinking: None,
        };
        assert_eq!(entry_to_value(&entry), json!({"model": "a/b"}));
        assert_eq!(entry_to_value(&RoutingEntry::default()), json!({}));
    }
}
