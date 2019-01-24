-- This will totally delete and clear out all data currently in the database
DROP DATABASE IF EXISTS kudos;
DROP USER IF EXISTS 'kudos'@'%';

CREATE DATABASE kudos;
CREATE USER 'kudos'@'%';
ALTER USER 'kudos'@'%' IDENTIFIED WITH mysql_native_password BY 'kudos';

USE kudos;

-- --

CREATE TABLE enabled_channels
(
  id      BIGINT AUTO_INCREMENT
    PRIMARY KEY,
  name    VARCHAR(255)   NOT NULL,
  enabled BOOL DEFAULT 1 NOT NULL,
  CONSTRAINT enabled_channels_name_uindex
    UNIQUE (name)
);

--

CREATE TABLE users
(
  id       BIGINT AUTO_INCREMENT
    PRIMARY KEY,
  slack_id VARCHAR(255) NOT NULL,
  username VARCHAR(255) NOT NULL,
  CONSTRAINT users_slack_id_uindex
    UNIQUE (slack_id),
  CONSTRAINT users_username_uindex
    UNIQUE (username)
);

--

CREATE TABLE kudos
(
  id        BIGINT AUTO_INCREMENT
    PRIMARY KEY,
  sender    BIGINT           NOT NULL,
  recipient BIGINT           NOT NULL,
  emoji     VARCHAR(255)     NOT NULL,
  count     BIGINT DEFAULT 0 NOT NULL,
  CONSTRAINT kudos_sender_recipient_emoji_uindex
    UNIQUE (sender, recipient, emoji),
  CONSTRAINT kudos_users_id_fk
    FOREIGN KEY (sender) REFERENCES users (id)
      ON UPDATE CASCADE ON DELETE CASCADE,
  CONSTRAINT kudos_users_id_fk_2
    FOREIGN KEY (recipient) REFERENCES users (id)
      ON UPDATE CASCADE ON DELETE CASCADE
);

--

CREATE TABLE rate
(
  id      BIGINT AUTO_INCREMENT
    PRIMARY KEY,
  user_id BIGINT                    NOT NULL,
  time    DATE,
  count   INT                       NOT NULL,
  CONSTRAINT rate_user_id_uindex
    UNIQUE (user_id),
  CONSTRAINT rate_users_id_fk
    FOREIGN KEY (user_id) REFERENCES users (id)
      ON UPDATE CASCADE ON DELETE CASCADE
);

DROP TRIGGER IF EXISTS rate_bi;
DELIMITER //
CREATE TRIGGER rate_bi
  BEFORE INSERT ON rate FOR EACH ROW
BEGIN
  IF (NEW.time IS NULL) THEN
    SET NEW.time = CURRENT_DATE();
  END IF;
END//
DELIMITER ;

--

GRANT ALL PRIVILEGES ON kudos.* to 'kudos'@'%';
FLUSH PRIVILEGES;
