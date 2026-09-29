use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DirectoryEntryDto {
    pub name: String,
    pub path: String,
    pub is_git_repo: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DirectoryListingDto {
    pub path: String,
    pub parent: Option<String>,
    pub entries: Vec<DirectoryEntryDto>,
}

#[cfg(feature = "ssr")]
mod mapping {
    use super::{DirectoryEntryDto, DirectoryListingDto};
    use crate::modules::workspaces::domain::models::directory_listing::{
        DirectoryEntry, DirectoryListing,
    };

    impl From<DirectoryEntry> for DirectoryEntryDto {
        fn from(entry: DirectoryEntry) -> Self {
            Self {
                name: entry.name,
                path: entry.path.to_string_lossy().into_owned(),
                is_git_repo: entry.is_git_repo,
            }
        }
    }

    impl From<DirectoryListing> for DirectoryListingDto {
        fn from(listing: DirectoryListing) -> Self {
            Self {
                path: listing.path.to_string_lossy().into_owned(),
                parent: listing
                    .parent
                    .map(|p: std::path::PathBuf| p.to_string_lossy().into_owned()),
                entries: listing
                    .entries
                    .into_iter()
                    .map(DirectoryEntryDto::from)
                    .collect(),
            }
        }
    }

    #[cfg(test)]
    mod tests {
        use std::path::PathBuf;

        use super::*;

        #[test]
        fn listing_maps_to_dto() {
            let listing: DirectoryListing = DirectoryListing {
                path: PathBuf::from("/home/pi"),
                parent: Some(PathBuf::from("/home")),
                entries: vec![DirectoryEntry {
                    name: "demo".to_string(),
                    path: PathBuf::from("/home/pi/demo"),
                    is_git_repo: true,
                }],
            };

            let dto: DirectoryListingDto = DirectoryListingDto::from(listing);

            assert_eq!(dto.path, "/home/pi");
            assert_eq!(dto.parent, Some("/home".to_string()));
            assert_eq!(dto.entries.len(), 1);
            assert_eq!(dto.entries[0].name, "demo");
            assert!(dto.entries[0].is_git_repo);
        }

        #[test]
        fn listing_without_parent_maps_to_none() {
            let listing: DirectoryListing = DirectoryListing {
                path: PathBuf::from("/"),
                parent: None,
                entries: vec![],
            };

            let dto: DirectoryListingDto = DirectoryListingDto::from(listing);

            assert_eq!(dto.parent, None);
        }
    }
}
