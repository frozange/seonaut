ALTER TABLE users
  DROP INDEX `users_verification_hash`,
  DROP COLUMN `verification_expires`,
  DROP COLUMN `verification_hash`,
  DROP COLUMN `email_verified`;
