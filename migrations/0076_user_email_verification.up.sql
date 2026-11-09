ALTER TABLE users
  ADD COLUMN `email_verified` tinyint NOT NULL DEFAULT 1,
  ADD COLUMN `verification_hash` char(64) NULL DEFAULT NULL,
  ADD COLUMN `verification_expires` datetime NULL DEFAULT NULL,
  ADD INDEX `users_verification_hash` (`verification_hash`);
