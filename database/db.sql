CREATE TYPE rewards_role AS ENUM (
    'Validator',
    'Delegator',
    'DAO',
    'Developers'
    );

CREATE TABLE addresses
(
    id      bigserial primary key,
    address character(40) NOT NULL unique
);

CREATE TABLE validators
(
    id                       serial primary key,
    reward_address_id        bigint references addresses (id) on delete cascade,
    owner_address_id         bigint references addresses (id) on delete cascade,
    control_address_id       bigint references addresses (id) on delete cascade,
    public_key               character varying(64) NOT NULL unique,
    created_at_block_id      integer,
    status                   integer,
    commission               integer,
    total_stake              numeric(70, 0),
    name                     varchar,
    description              varchar,
    site_url                 varchar,
    icon_url                 varchar,
    meta_updated_at_block_id bigint,
    baned_till               bigint                   default null,
    update_at                timestamp with time zone DEFAULT current_timestamp
);

CREATE TABLE validator_public_keys
(
    id           serial primary key,
    validator_id integer references validators (id) on delete cascade,
    key          character varying(64) NOT NULL unique,
    created_at   timestamp with time zone DEFAULT current_timestamp,
    update_at    timestamp with time zone DEFAULT null
);
CREATE INDEX validator_public_keys_validator_id_index ON validator_public_keys USING btree (validator_id);
CREATE INDEX validator_public_keys_key_index ON validator_public_keys USING btree (key);

CREATE TABLE blocks
(
    id                    bigint                   NOT NULL unique,
    size                  integer                  NOT NULL,
    proposer_validator_id integer                  NOT NULL references validators (id) on delete cascade,
    num_txs               integer                  NOT NULL DEFAULT 0,
    block_time            bigint                   NOT NULL,
    created_at            timestamp with time zone NOT NULL,
    updated_at            timestamp with time zone NOT NULL DEFAULT current_timestamp,
    block_reward          numeric(70, 0)           NOT NULL,
    hash                  character varying(64)    NOT NULL
);
CREATE INDEX blocks_proposer_validator_id_index ON blocks USING btree (proposer_validator_id);
CREATE INDEX blocks_created_at_index ON blocks (created_at DESC);

CREATE TABLE block_validator
(
    block_id     bigint  NOT NULL references blocks (id) on delete cascade,
    validator_id integer NOT NULL references validators (id) on delete cascade,
    signed       boolean NOT NULL DEFAULT false
);
CREATE INDEX block_validator_block_id_index ON block_validator USING btree (block_id);
CREATE INDEX block_validator_validator_id_index ON block_validator USING btree (validator_id);

CREATE TABLE coins
(
    id                  serial primary key,
    type                integer,
    name                character varying(255),
    symbol              character varying(20) NOT NULL,
    volume              numeric(70, 0),
    crr                 integer,
    reserve             numeric(70, 0),
    max_supply          numeric(70, 0),
    version             integer,
    owner_address_id    bigint REFERENCES addresses (id) on delete cascade,
    created_at_block_id bigint,
    burnable            boolean,
    mintable            boolean,
    created_at          timestamp with time zone DEFAULT current_timestamp,
    updated_at          timestamp with time zone DEFAULT NULL,
    deleted_at          timestamp with time zone DEFAULT NULL,
    UNIQUE (symbol, version)
);
CREATE INDEX coins_symbol_index ON coins USING btree (symbol);

CREATE TABLE balances
(
    address_id bigint         NOT NULL REFERENCES addresses (id) on delete cascade,
    coin_id    integer        NOT NULL REFERENCES coins (id) on delete cascade,
    value      numeric(70, 0) NOT NULL,
    UNIQUE (address_id, coin_id)
);
CREATE INDEX balances_address_id_index ON balances USING btree (address_id);
CREATE INDEX balances_coin_id_index ON balances USING btree (coin_id);

CREATE TABLE transactions
(
    id              bigserial primary key,
    from_address_id bigint                   NOT NULL references addresses (id) on delete cascade,
    nonce           bigint                   NOT NULL,
    gas_price       bigint                   NOT NULL,
    gas             bigint                   NOT NULL,
    commission      numeric(70, 0),
    block_id        integer                  NOT NULL references blocks (id) on delete cascade,
    gas_coin_id     integer                  NOT NULL references coins (id) on delete cascade,
    created_at      timestamp with time zone NOT NULL,
    type            smallint                 NOT NULL,
    hash            character varying(64)    NOT NULL,
    service_data    text,
    data            jsonb                    NOT NULL,
    tags            jsonb                    NOT NULL,
    payload         bytea,
    raw_tx          bytea                    NOT NULL
);
CREATE INDEX transactions_block_id_from_address_id_index ON transactions USING btree (block_id DESC, from_address_id);
CREATE INDEX transactions_from_address_id_index ON transactions USING btree (from_address_id);
CREATE INDEX transactions_hash_index ON transactions USING hash (hash);

CREATE TABLE invalid_transactions
(
    id              bigserial primary key,
    from_address_id bigint                   NOT NULL references addresses (id) on delete cascade,
    block_id        integer                  NOT NULL references blocks (id) on delete cascade,
    created_at      timestamp with time zone NOT NULL,
    type            smallint                 NOT NULL,
    hash            character varying(64)    NOT NULL,
    log             character varying,
    tx_data         jsonb                    NOT NULL
);
CREATE INDEX invalid_transactions_block_id_from_address_id_index ON invalid_transactions USING btree (block_id DESC, from_address_id);
CREATE INDEX invalid_transactions_from_address_id_index ON invalid_transactions USING btree (from_address_id);
CREATE INDEX invalid_transactions_hash_index ON invalid_transactions USING hash (hash);

CREATE TABLE transaction_outputs
(
    id             bigserial primary key,
    transaction_id bigint         NOT NULL references transactions (id) on delete cascade,
    to_address_id  bigint         NOT NULL references addresses (id) on delete cascade,
    coin_id        integer        NOT NULL references coins (id) on delete cascade,
    value          numeric(70, 0) NOT NULL
);
CREATE INDEX transaction_outputs_coin_id_index ON transaction_outputs USING btree (coin_id);
CREATE INDEX transaction_outputs_transaction_id_index ON transaction_outputs USING btree (transaction_id);
CREATE INDEX transaction_outputs_address_id_index ON transaction_outputs USING btree (to_address_id);

CREATE TABLE transaction_validator
(
    transaction_id bigint  NOT NULL references transactions (id) on delete cascade,
    validator_id   integer NOT NULL references validators (id) on delete cascade
);
CREATE INDEX transaction_validator_validator_id_index ON transaction_validator USING btree (validator_id);

CREATE TABLE index_transaction_by_address
(
    block_id       bigint NOT NULL references blocks (id) on delete cascade,
    address_id     bigint NOT NULL references addresses (id) on delete cascade,
    transaction_id bigint NOT NULL references transactions (id) on delete cascade,
    unique (block_id, address_id, transaction_id)
);

CREATE INDEX index_transaction_by_address_address_id_index ON index_transaction_by_address USING btree (address_id);
CREATE INDEX index_transaction_by_address_block_id_address_id_index ON index_transaction_by_address USING btree (block_id, address_id);
CREATE INDEX index_transaction_by_address_transaction_id_index ON index_transaction_by_address USING btree (transaction_id);

CREATE TABLE aggregated_rewards
(
    time_id       timestamp with time zone NOT NULL,
    to_block_id   integer                  NOT NULL references blocks (id) on delete cascade,
    from_block_id integer                  NOT NULL references blocks (id) on delete cascade,
    address_id    bigint                   NOT NULL references addresses (id) on delete cascade,
    validator_id  integer                  NOT NULL references validators (id) on delete cascade,
    role          rewards_role             NOT NULL,
    amount        numeric(70, 0)           NOT NULL
);
CREATE INDEX aggregated_rewards_address_id_index ON aggregated_rewards USING btree (address_id);
CREATE INDEX aggregated_rewards_validator_id_index ON aggregated_rewards USING btree (validator_id);
CREATE INDEX aggregated_rewards_time_id_index ON aggregated_rewards USING btree (time_id);
CREATE UNIQUE INDEX aggregated_rewards_unique_index ON aggregated_rewards
    USING btree (time_id, address_id, validator_id, role);

CREATE TABLE slashes
(
    id           bigserial      NOT NULL,
    address_id   bigint         NOT NULL references addresses (id) on delete cascade,
    block_id     integer        NOT NULL references blocks (id) on delete cascade,
    validator_id integer        NOT NULL references validators (id) on delete cascade,
    coin_id      integer        NOT NULL references coins (id) on delete cascade,
    amount       numeric(70, 0) NOT NULL
);
CREATE INDEX slashes_address_id_index ON slashes USING btree (address_id);
CREATE INDEX slashes_block_id_index ON slashes USING btree (block_id);
CREATE INDEX slashes_coin_id_index ON slashes USING btree (coin_id);
CREATE INDEX slashes_validator_id_index ON slashes USING btree (validator_id);

CREATE TABLE stakes
(
    id               serial         NOT NULL,
    owner_address_id bigint         NOT NULL references addresses (id) on delete cascade,
    validator_id     integer        NOT NULL references validators (id) on delete cascade,
    coin_id          integer        NOT NULL references coins (id) on delete cascade,
    value            numeric(70, 0) NOT NULL,
    bip_value        numeric(70, 0) NOT NULL,
    is_kicked        bool default false,
    UNIQUE (owner_address_id, validator_id, coin_id)
);
CREATE INDEX stakes_coin_id_index ON stakes USING btree (coin_id);
CREATE INDEX stakes_owner_address_id_index ON stakes USING btree (owner_address_id);
CREATE INDEX stakes_validator_id_index ON stakes USING btree (validator_id);

CREATE TABLE unbonds
(
    block_id     bigint         NOT NULL,
    address_id   bigint         NOT NULL references addresses (id) on delete cascade,
    coin_id      integer        NOT NULL references coins (id) on delete cascade,
    validator_id integer        NOT NULL references validators (id) on delete cascade,
    value        numeric(70, 0) NOT NULL,
    created_at   timestamp with time zone DEFAULT current_timestamp
);

CREATE INDEX unbonds_address_id_index ON unbonds USING btree (address_id);
CREATE INDEX unbonds_coin_id_index ON unbonds USING btree (coin_id);
CREATE INDEX unbonds_validator_id_index ON unbonds USING btree (validator_id);

CREATE TABLE checks
(
    transaction_id  bigint NOT NULL references transactions (id) on delete cascade,
    from_address_id bigint NOT NULL references addresses (id) on delete cascade,
    to_address_id   bigint NOT NULL references addresses (id) on delete cascade,
    data            varchar
);
CREATE INDEX checks_transaction_id_index ON checks USING btree (transaction_id);
CREATE INDEX checks_from_address_id_index ON checks USING btree (from_address_id);
CREATE INDEX checks_to_address_id_index ON checks USING btree (to_address_id);
CREATE INDEX checks_check_index ON checks USING btree (data);

CREATE TABLE liquidity_pools
(
    id                 serial primary key,
    token_id           integer         NOT NULL references coins (id) on delete cascade,
    first_coin_id      integer         NOT NULL references coins (id) on delete cascade,
    second_coin_id     integer         NOT NULL references coins (id) on delete cascade,
    first_coin_volume  numeric(100, 0) NOT NULL,
    second_coin_volume numeric(100, 0) NOT NULL,
    liquidity          numeric(100, 0) NOT NULL,
    unique (first_coin_id, second_coin_id)
);
CREATE INDEX liquidity_pools_first_coin_id_index ON liquidity_pools USING btree (first_coin_id);
CREATE INDEX liquidity_pools_second_coin_id_index ON liquidity_pools USING btree (second_coin_id);

CREATE TABLE address_liquidity_pools
(
    address_id        bigint          not null references addresses (id) on delete cascade,
    liquidity_pool_id int             not null references liquidity_pools (id) on delete cascade,
    liquidity         numeric(100, 0) not null,
    unique (address_id, liquidity_pool_id)
);

CREATE INDEX address_liquidity_address_id_index ON address_liquidity_pools USING btree (address_id);
CREATE INDEX address_liquidity_liquidity_pool_id_index ON address_liquidity_pools USING btree (liquidity_pool_id);

CREATE TABLE transaction_liquidity_pool
(
    transaction_id    bigint not null references transactions (id) on delete cascade,
    liquidity_pool_id int    not null references liquidity_pools (id) on delete cascade,
    unique (transaction_id, liquidity_pool_id)
);
CREATE INDEX transaction_liquidity_pool_tx_id_index ON transaction_liquidity_pool USING btree (transaction_id);
CREATE INDEX transaction_liquidity_pool_lp_id_index ON transaction_liquidity_pool USING btree (liquidity_pool_id);

CREATE TABLE liquidity_pool_trades
(
    block_id           bigint          not null references blocks (id) on delete cascade,
    liquidity_pool_id  bigint          not null references liquidity_pools (id) on delete cascade,
    transaction_id     bigint          not null references transactions (id) on delete cascade,
    first_coin_volume  numeric(100, 0) not null,
    second_coin_volume numeric(100, 0) not null,
    created_at         timestamp with time zone DEFAULT current_timestamp
);

CREATE INDEX pool_trades_block_id_index ON liquidity_pool_trades USING btree (block_id);
CREATE INDEX pool_trades_liquidity_pool_id_index ON liquidity_pool_trades USING btree (liquidity_pool_id);
CREATE INDEX pool_trades_transaction_id_index ON liquidity_pool_trades USING btree (transaction_id);
