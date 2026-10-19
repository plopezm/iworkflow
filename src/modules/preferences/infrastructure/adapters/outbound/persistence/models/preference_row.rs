use diesel::prelude::*;

use crate::core::db::schema::app_preferences;

#[derive(Debug, Clone, Queryable, Selectable, Insertable)]
#[diesel(table_name = app_preferences)]
#[diesel(check_for_backend(diesel::sqlite::Sqlite))]
pub struct PreferenceRow {
    pub key: String,
    pub value: String,
}
