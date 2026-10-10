use diesel::SqliteConnection;
use diesel_async::pooled_connection::deadpool::{BuildError, Object, Pool, PoolError};
use diesel_async::pooled_connection::{AsyncDieselConnectionManager, ManagerConfig};
use diesel_async::sync_connection_wrapper::SyncConnectionWrapper;
use diesel_async::{AsyncConnection, AsyncMigrationHarness, SimpleAsyncConnection};
use diesel_migrations::{EmbeddedMigrations, MigrationHarness, embed_migrations};
use futures::FutureExt;
use futures::future::BoxFuture;

pub type DbConnectionRaw = SyncConnectionWrapper<SqliteConnection>;
pub type DbPool = Pool<DbConnectionRaw>;
pub type DbConnection = Object<DbConnectionRaw>;

pub const MIGRATIONS: EmbeddedMigrations = embed_migrations!("migrations");

/// Ajustes aplicados a cada conexión SQLite nueva.
const CONNECTION_PRAGMAS: &str = "PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000; \
     PRAGMA synchronous = NORMAL; PRAGMA temp_store = MEMORY;";

#[derive(Debug, thiserror::Error)]
pub enum DbError {
    #[error("no se pudo construir el pool: {0}")]
    Build(#[from] BuildError),
    #[error("no se pudo obtener conexión: {0}")]
    Pool(#[from] PoolError),
    #[error("fallo ejecutando migraciones: {0}")]
    Migration(String),
    #[error("no se pudo preparar la carpeta de la base de datos: {0}")]
    Io(#[from] std::io::Error),
}

fn establish_with_pragmas(url: &str) -> BoxFuture<'_, diesel::ConnectionResult<DbConnectionRaw>> {
    async move {
        let mut connection: DbConnectionRaw = DbConnectionRaw::establish(url).await?;
        connection
            .batch_execute(CONNECTION_PRAGMAS)
            .await
            .map_err(diesel::ConnectionError::CouldntSetupConfiguration)?;
        Ok(connection)
    }
    .boxed()
}

/// Construye el pool de SQLite creando la carpeta contenedora si hace falta.
pub fn build_pool(database_url: &str) -> Result<DbPool, DbError> {
    if let Some(parent) = std::path::Path::new(database_url).parent()
        && !parent.as_os_str().is_empty()
    {
        std::fs::create_dir_all(parent)?;
    }
    let mut config: ManagerConfig<DbConnectionRaw> = ManagerConfig::default();
    config.custom_setup = Box::new(establish_with_pragmas);
    let manager: AsyncDieselConnectionManager<DbConnectionRaw> =
        AsyncDieselConnectionManager::<DbConnectionRaw>::new_with_config(database_url, config);
    let pool: DbPool = Pool::builder(manager).build()?;
    Ok(pool)
}

/// Aplica las migraciones pendientes. Requiere runtime tokio multihilo.
pub async fn run_migrations(pool: &DbPool) -> Result<(), DbError> {
    let connection: DbConnection = pool.get().await?;
    let mut harness: AsyncMigrationHarness<DbConnection> = AsyncMigrationHarness::new(connection);
    harness
        .run_pending_migrations(MIGRATIONS)
        .map_err(|e| DbError::Migration(e.to_string()))?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use diesel::QueryableByName;
    use diesel::sql_types::Text;
    use diesel_async::RunQueryDsl;

    #[derive(QueryableByName)]
    struct IntPragma {
        #[diesel(sql_type = diesel::sql_types::BigInt)]
        value: i64,
    }

    #[derive(QueryableByName)]
    struct TextPragma {
        #[diesel(sql_type = Text)]
        value: String,
    }

    #[tokio::test(flavor = "multi_thread")]
    async fn pooled_connection_applies_performance_pragmas() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let url: String = dir.path().join("pragmas.db").to_string_lossy().into_owned();
        let pool: DbPool = build_pool(&url).expect("pool");
        let mut conn: DbConnection = pool.get().await.expect("conn");

        let synchronous: Vec<IntPragma> =
            diesel::sql_query("SELECT synchronous AS value FROM pragma_synchronous")
                .load(&mut conn)
                .await
                .expect("synchronous");
        let temp_store: Vec<IntPragma> =
            diesel::sql_query("SELECT temp_store AS value FROM pragma_temp_store")
                .load(&mut conn)
                .await
                .expect("temp_store");
        let journal: Vec<TextPragma> =
            diesel::sql_query("SELECT journal_mode AS value FROM pragma_journal_mode")
                .load(&mut conn)
                .await
                .expect("journal");

        assert_eq!(synchronous[0].value, 1, "NORMAL");
        assert_eq!(temp_store[0].value, 2, "MEMORY");
        assert_eq!(journal[0].value, "wal");
    }
}
