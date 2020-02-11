--
-- PostgreSQL database dump
--

-- Dumped from database version 9.5.14
-- Dumped by pg_dump version 9.5.14

SET statement_timeout = 0;
SET lock_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: plpgsql; Type: EXTENSION; Schema: -; Owner:
--

CREATE EXTENSION IF NOT EXISTS plpgsql WITH SCHEMA pg_catalog;


--
-- Name: EXTENSION plpgsql; Type: COMMENT; Schema: -; Owner:
--

COMMENT ON EXTENSION plpgsql IS 'PL/pgSQL procedural language';


--
-- Name: rewards_role; Type: TYPE; Schema: public; Owner: minter
--

CREATE TYPE public.rewards_role AS ENUM (
    'Validator',
    'Delegator',
    'DAO',
    'Developers'
    );



SET default_tablespace = '';

SET default_with_oids = false;

--
-- Name: addresses; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.addresses
(
    id                  bigint                NOT NULL,
    address             character varying(40) NOT NULL,
    updated_at          timestamp with time zone,
    updated_at_block_id bigint
);



--
-- Name: COLUMN addresses.address; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.addresses.address IS 'Address hex string without prefix(Mx****)';


--
-- Name: COLUMN addresses.updated_at; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.addresses.updated_at IS 'Last balance parsing time';

--
-- Name: COLUMN addresses.updated_at_block_id; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.addresses.updated_at_block_id IS 'Block id, that have transactions or events, that triggers address record to update from api-method GET /address';


--
-- Name: addresses_id_seq; Type: SEQUENCE; Schema: public; Owner: minter
--

CREATE SEQUENCE public.addresses_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



--
-- Name: addresses_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: minter
--

ALTER SEQUENCE public.addresses_id_seq OWNED BY public.addresses.id;


--
-- Name: balances; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.balances
(
    id         bigint         NOT NULL,
    address_id bigint         NOT NULL,
    coin_id    integer        NOT NULL,
    value      numeric(70, 0) NOT NULL
);



--
-- Name: balances_id_seq; Type: SEQUENCE; Schema: public; Owner: minter
--

CREATE SEQUENCE public.balances_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



--
-- Name: balances_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: minter
--

ALTER SEQUENCE public.balances_id_seq OWNED BY public.balances.id;


--
-- Name: block_validator; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.block_validator
(
    block_id     bigint                NOT NULL,
    validator_id integer               NOT NULL,
    signed       boolean DEFAULT false NOT NULL
);



--
-- Name: blocks; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.blocks
(
    id                    integer                  NOT NULL,
    total_txs             bigint                   NOT NULL DEFAULT 0,
    size                  bigint                   NOT NULL,
    proposer_validator_id integer                  NOT NULL,
    num_txs               integer                  NOT NULL DEFAULT 0,
    block_time            bigint                   NOT NULL,
    created_at            timestamp with time zone NOT NULL,
    updated_at            timestamp with time zone          DEFAULT now() NOT NULL,
    block_reward          numeric(70, 0)           NOT NULL,
    hash                  character varying(64)    NOT NULL
);



--
-- Name: TABLE blocks; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON TABLE public.blocks IS 'Address entity table';


--
-- Name: COLUMN blocks.total_txs; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.blocks.total_txs IS 'Total count of txs in blockchain';


--
-- Name: COLUMN blocks.proposer_validator_id; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.blocks.proposer_validator_id IS 'Proposer public key (Mp***)';


--
-- Name: COLUMN blocks.num_txs; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.blocks.num_txs IS 'Count of txs in block';


--
-- Name: COLUMN blocks.block_time; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.blocks.block_time IS 'Block operation time (???) in microseconds';


--
-- Name: COLUMN blocks.created_at; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.blocks.created_at IS 'Datetime of block creation("time" field from api)';


--
-- Name: COLUMN blocks.updated_at; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.blocks.updated_at IS 'Time of record last update';


--
-- Name: COLUMN blocks.block_reward; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.blocks.block_reward IS 'Sum of all block rewards';


--
-- Name: COLUMN blocks.hash; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.blocks.hash IS 'Hex string';


--
-- Name: coins; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.coins
(
    id                      integer                                NOT NULL,
    creation_address_id     bigint,
    creation_transaction_id bigint,
    crr                     integer,
    volume                  numeric(70, 0),
    reserve_balance         numeric(70, 0),
    max_supply              numeric(70, 0),
    name                    character varying(255),
    symbol                  character varying(20)                  NOT NULL,
    updated_at              timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at              timestamp with time zone               NULL
);



--
-- Name: COLUMN coins.creation_address_id; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.coins.creation_address_id IS 'Id of creator address in address table';


--
-- Name: COLUMN coins.updated_at; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.coins.updated_at IS 'Timestamp of coin balance/value updation(from api for example)';


--
-- Name: COLUMN coins.reserve_balance; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.coins.reserve_balance IS 'Reservation balance for coin creation
';


--
-- Name: COLUMN coins.name; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.coins.name IS 'Name of coin';


--
-- Name: COLUMN coins.symbol; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.coins.symbol IS 'Short symbol of coin';


--
-- Name: coins_id_seq; Type: SEQUENCE; Schema: public; Owner: minter
--

CREATE SEQUENCE public.coins_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



--
-- Name: coins_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: minter
--

ALTER SEQUENCE public.coins_id_seq OWNED BY public.coins.id;


--
-- Name: invalid_transactions; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.invalid_transactions
(
    id              bigint                   NOT NULL,
    from_address_id bigint                   NOT NULL,
    block_id        integer                  NOT NULL,
    created_at      timestamp with time zone NOT NULL,
    type            smallint                 NOT NULL,
    hash            character varying(64)    NOT NULL,
    tx_data         jsonb                    NOT NULL
);



--
-- Name: COLUMN invalid_transactions.created_at; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.invalid_transactions.created_at IS 'Duplicate of block created_at for less joins listings';


--
-- Name: invalid_transactions_id_seq; Type: SEQUENCE; Schema: public; Owner: minter
--

CREATE SEQUENCE public.invalid_transactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



--
-- Name: invalid_transactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: minter
--

ALTER SEQUENCE public.invalid_transactions_id_seq OWNED BY public.invalid_transactions.id;


--
-- Name: rewards; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.rewards
(
    address_id   bigint              NOT NULL,
    block_id     integer             NOT NULL,
    validator_id integer             NOT NULL,
    role         public.rewards_role NOT NULL,
    amount       numeric(70, 0)      NOT NULL
);



--
-- Name: aggregated_rewards; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.aggregated_rewards
(
    time_id       timestamp with time zone NOT NULL,
    to_block_id   integer                  NOT NULL,
    from_block_id integer                  NOT NULL,
    address_id    bigint                   NOT NULL,
    validator_id  integer                  NOT NULL,
    role          public.rewards_role      NOT NULL,
    amount        numeric(70, 0)           NOT NULL
);



--
-- Name: slashes; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.slashes
(
    id           bigint         NOT NULL,
    address_id   bigint         NOT NULL,
    block_id     integer        NOT NULL,
    validator_id integer        NOT NULL,
    coin_id      integer        NOT NULL,
    amount       numeric(70, 0) NOT NULL
);



--
-- Name: slashes_id_seq; Type: SEQUENCE; Schema: public; Owner: minter
--

CREATE SEQUENCE public.slashes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



--
-- Name: slashes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: minter
--

ALTER SEQUENCE public.slashes_id_seq OWNED BY public.slashes.id;


--
-- Name: stakes; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.stakes
(
    id               serial         NOT NULL,
    owner_address_id bigint         NOT NULL,
    validator_id     integer        NOT NULL,
    coin_id          integer        NOT NULL,
    value            numeric(70, 0) NOT NULL,
    bip_value        numeric(70, 0) NOT NULL
);

--
-- Name: transaction_outputs; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.transaction_outputs
(
    id             bigint         NOT NULL,
    transaction_id bigint         NOT NULL,
    to_address_id  bigint         NOT NULL,
    coin_id        integer        NOT NULL,
    value          numeric(70, 0) NOT NULL
);



--
-- Name: COLUMN transaction_outputs.value; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.transaction_outputs.value IS 'Value of tx output';


--
-- Name: transaction_outputs_id_seq; Type: SEQUENCE; Schema: public; Owner: minter
--

CREATE SEQUENCE public.transaction_outputs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



--
-- Name: transaction_outputs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: minter
--

ALTER SEQUENCE public.transaction_outputs_id_seq OWNED BY public.transaction_outputs.id;


--
-- Name: transaction_validator; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.transaction_validator
(
    transaction_id bigint  NOT NULL,
    validator_id   integer NOT NULL
);



--
-- Name: transactions; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.transactions
(
    id              bigint                   NOT NULL,
    from_address_id bigint                   NOT NULL,
    nonce           bigint                   NOT NULL,
    gas_price       bigint                   NOT NULL,
    gas             bigint                   NOT NULL,
    block_id        integer                  NOT NULL,
    gas_coin_id     integer                  NOT NULL,
    created_at      timestamp with time zone NOT NULL,
    type            smallint                 NOT NULL,
    hash            character varying(64)    NOT NULL,
    service_data    text,
    data            jsonb                    NOT NULL,
    tags            jsonb                    NOT NULL,
    payload         bytea,
    raw_tx          bytea                    NOT NULL
);



--
-- Name: COLUMN transactions.from_address_id; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.transactions.from_address_id IS 'Link to address, from that tx was signed';


--
-- Name: COLUMN transactions.block_id; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.transactions.block_id IS 'Link to block';


--
-- Name: COLUMN transactions.created_at; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.transactions.created_at IS 'Timestamp of tx = timestamp of block. Duplicate data for less joins on blocks';


--
-- Name: COLUMN transactions.type; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.transactions.type IS 'Integer index of tx type';

--
-- Name: COLUMN transactions.hash; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.transactions.hash IS 'Tx hash 64 symbols hex string without prefix(Mt****). Because of key-value-only filtering uses hash index';


--
-- Name: COLUMN transactions.payload; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.transactions.payload IS 'transaction payload in bytes';


--
-- Name: COLUMN transactions.raw_tx; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON COLUMN public.transactions.raw_tx IS 'Raw tx data in bytes';


--
-- Name: transactions_id_seq; Type: SEQUENCE; Schema: public; Owner: minter
--

CREATE SEQUENCE public.transactions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



--
-- Name: transactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: minter
--

ALTER SEQUENCE public.transactions_id_seq OWNED BY public.transactions.id;


--
-- Name: validators; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.validators
(
    id                       integer                                NOT NULL,
    reward_address_id        bigint,
    owner_address_id         bigint,
    created_at_block_id      integer,
    status                   integer,
    commission               integer,
    total_stake              numeric(70, 0),
    public_key               character varying(64)                  NOT NULL,
    name                     varchar,
    description              varchar,
    site_url                 varchar,
    icon_url                 varchar,
    meta_updated_at_block_id bigint,
    update_at                timestamp with time zone DEFAULT now() NOT NULL
);



--
-- Name: TABLE validators; Type: COMMENT; Schema: public; Owner: minter
--

COMMENT ON TABLE public.validators IS 'ATTENTION - only public _ey is not null field, other fields can be null';


--
-- Name: validator_public_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: minter
--

CREATE SEQUENCE public.validator_public_keys_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;



--
-- Name: validator_public_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: minter
--

ALTER SEQUENCE public.validator_public_keys_id_seq OWNED BY public.validators.id;


--
-- Name: index_transaction_by_address; Type: TABLE; Schema: public; Owner: minter
--

CREATE TABLE public.index_transaction_by_address
(
    block_id       bigint NOT NULL,
    address_id     bigint NOT NULL,
    transaction_id bigint NOT NULL
);

--
-- Name: id; Type: DEFAULT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.addresses
    ALTER COLUMN id SET DEFAULT nextval('public.addresses_id_seq'::regclass);


--
-- Name: id; Type: DEFAULT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.balances
    ALTER COLUMN id SET DEFAULT nextval('public.balances_id_seq'::regclass);


--
-- Name: id; Type: DEFAULT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.coins
    ALTER COLUMN id SET DEFAULT nextval('public.coins_id_seq'::regclass);


--
-- Name: id; Type: DEFAULT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.invalid_transactions
    ALTER COLUMN id SET DEFAULT nextval('public.invalid_transactions_id_seq'::regclass);


--
-- Name: id; Type: DEFAULT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.slashes
    ALTER COLUMN id SET DEFAULT nextval('public.slashes_id_seq'::regclass);

--
-- Name: id; Type: DEFAULT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transaction_outputs
    ALTER COLUMN id SET DEFAULT nextval('public.transaction_outputs_id_seq'::regclass);


--
-- Name: id; Type: DEFAULT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transactions
    ALTER COLUMN id SET DEFAULT nextval('public.transactions_id_seq'::regclass);


--
-- Name: id; Type: DEFAULT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.validators
    ALTER COLUMN id SET DEFAULT nextval('public.validator_public_keys_id_seq'::regclass);


--
-- Data for Name: addresses; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.addresses (id, address, updated_at) FROM stdin;
\.


--
-- Name: addresses_id_seq; Type: SEQUENCE SET; Schema: public; Owner: minter
--

SELECT pg_catalog.setval('public.addresses_id_seq', 1, false);


--
-- Data for Name: balances; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.balances (id, address_id, coin_id, value) FROM stdin;
\.


--
-- Name: balances_id_seq; Type: SEQUENCE SET; Schema: public; Owner: minter
--

SELECT pg_catalog.setval('public.balances_id_seq', 1, false);


--
-- Data for Name: block_validator; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.block_validator (block_id, validator_id, signed) FROM stdin;
\.


--
-- Data for Name: blocks; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.blocks (id, total_txs, size, proposer_validator_id, num_txs, block_time, created_at, updated_at,
                    block_reward, hash) FROM stdin;
\.


--
-- Data for Name: coins; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.coins (id, creation_address_id, creation_transaction_id, crr, updated_at, volume,
                   reserve_balance, name, symbol) FROM stdin;
\.


--
-- Name: coins_id_seq; Type: SEQUENCE SET; Schema: public; Owner: minter
--

SELECT pg_catalog.setval('public.coins_id_seq', 1, false);


--
-- Data for Name: invalid_transactions; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.invalid_transactions (id, from_address_id, block_id, created_at, type, hash, tx_data) FROM stdin;
\.


--
-- Name: invalid_transactions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: minter
--

SELECT pg_catalog.setval('public.invalid_transactions_id_seq', 1, false);


--
-- Data for Name: rewards; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.rewards (address_id, block_id, validator_id, role, amount) FROM stdin;
\.


--
-- Data for Name: slashes; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.slashes (id, address_id, block_id, validator_id, coin_id, amount) FROM stdin;
\.


--
-- Name: slashes_id_seq; Type: SEQUENCE SET; Schema: public; Owner: minter
--

SELECT pg_catalog.setval('public.slashes_id_seq', 1, false);


--
-- Data for Name: stakes; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.stakes (owner_address_id, validator_id, coin_id, value, bip_value) FROM stdin;
\.

--
-- Data for Name: transaction_outputs; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.transaction_outputs (id, transaction_id, to_address_id, coin_id, value) FROM stdin;
\.


--
-- Name: transaction_outputs_id_seq; Type: SEQUENCE SET; Schema: public; Owner: minter
--

SELECT pg_catalog.setval('public.transaction_outputs_id_seq', 1, false);


--
-- Data for Name: transaction_validator; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.transaction_validator (transaction_id, validator_id) FROM stdin;
\.


--
-- Data for Name: transactions; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.transactions (id, from_address_id, nonce, gas_price, gas, block_id, gas_coin_id, created_at, type, hash,
                          service_data, data, tags, payload, raw_tx) FROM stdin;
\.


--
-- Name: transactions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: minter
--

SELECT pg_catalog.setval('public.transactions_id_seq', 1, false);


--
-- Name: validator_public_keys_id_seq; Type: SEQUENCE SET; Schema: public; Owner: minter
--

SELECT pg_catalog.setval('public.validator_public_keys_id_seq', 1, false);


--
-- Data for Name: validators; Type: TABLE DATA; Schema: public; Owner: minter
--

COPY public.validators (id, reward_address_id, owner_address_id, created_at_block_id, status, commission, total_stake,
                        public_key, update_at) FROM stdin;
\.


--
-- Name: addresses_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.addresses
    ADD CONSTRAINT addresses_pkey PRIMARY KEY (id);


--
-- Name: balances_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.balances
    ADD CONSTRAINT balances_pkey PRIMARY KEY (id);


--
-- Name: block_validator_pk; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.block_validator
    ADD CONSTRAINT block_validator_pk PRIMARY KEY (block_id, validator_id);


--
-- Name: blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.blocks
    ADD CONSTRAINT blocks_pkey PRIMARY KEY (id);


--
-- Name: coins_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.coins
    ADD CONSTRAINT coins_pkey PRIMARY KEY (id);


--
-- Name: invalid_transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.invalid_transactions
    ADD CONSTRAINT invalid_transactions_pkey PRIMARY KEY (id);

--
-- Name: slashes_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.slashes
    ADD CONSTRAINT slashes_pkey PRIMARY KEY (id);


--
-- Name: stakes_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.stakes
    ADD CONSTRAINT stakes_pkey PRIMARY KEY (validator_id, owner_address_id, coin_id);


--
-- Name: transaction_outputs_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transaction_outputs
    ADD CONSTRAINT transaction_outputs_pkey PRIMARY KEY (id);


--
-- Name: transaction_validator_pk; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transaction_validator
    ADD CONSTRAINT transaction_validator_pk PRIMARY KEY (transaction_id, validator_id);


--
-- Name: transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_pkey PRIMARY KEY (id);


--
-- Name: validator_public_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.validators
    ADD CONSTRAINT validator_public_keys_pkey PRIMARY KEY (id);


--
-- Name: index_transaction_by_address index_transaction_by_address_pk; Type: CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.index_transaction_by_address
    ADD CONSTRAINT index_transaction_by_address_pk PRIMARY KEY (block_id, address_id, transaction_id);


--
-- Name: addresses_address_uindex; Type: INDEX; Schema: public; Owner: minter
--

CREATE UNIQUE INDEX addresses_address_uindex ON public.addresses USING btree (address);


--
-- Name: balances_address_id_coind_id_uindex; Type: INDEX; Schema: public; Owner: minter
--

CREATE UNIQUE INDEX balances_address_id_coind_id_uindex ON public.balances USING btree (address_id, coin_id);


--
-- Name: balances_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX balances_address_id_index ON public.balances USING btree (address_id);


--
-- Name: balances_coind_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX balances_coind_id_index ON public.balances USING btree (coin_id);


--
-- Name: block_validator_block_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX block_validator_block_id_index ON public.block_validator USING btree (block_id);


--
-- Name: block_validator_validator_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX block_validator_validator_id_index ON public.block_validator USING btree (validator_id);


--
-- Name: blocks_proposer_validator_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX blocks_proposer_validator_id_index ON public.blocks USING btree (proposer_validator_id);


--
-- Name: blocks_proposer_validator_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX blocks_created_at_index ON public.blocks (created_at DESC);


--
-- Name: coins_creation_transaction_id_uindex; Type: INDEX; Schema: public; Owner: minter
--

CREATE UNIQUE INDEX coins_creation_transaction_id_uindex ON public.coins USING btree (creation_transaction_id);


--
-- Name: coins_creator_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX coins_creator_address_id_index ON public.coins USING btree (creation_address_id);

--
-- Name: coins_symbol_uindex; Type: UNIQUE INDEX; Schema: public; Owner: minter
--

CREATE UNIQUE INDEX coins_symbol_uindex ON public.coins (symbol);


--
-- Name: invalid_transactions_block_id_from_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX invalid_transactions_block_id_from_address_id_index ON public.invalid_transactions USING btree (block_id DESC, from_address_id);


--
-- Name: invalid_transactions_from_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX invalid_transactions_from_address_id_index ON public.invalid_transactions USING btree (from_address_id);


--
-- Name: invalid_transactions_hash_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX invalid_transactions_hash_index ON public.invalid_transactions USING hash (hash);


--
-- Name: rewards_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX rewards_address_id_index ON public.rewards USING btree (address_id);


--
-- Name: rewards_block_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX rewards_block_id_index ON public.rewards USING btree (block_id);


--
-- Name: rewards_validator_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX rewards_validator_id_index ON public.rewards USING btree (validator_id);


--
-- Name: aggregated_rewards_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX aggregated_rewards_address_id_index ON public.aggregated_rewards USING btree (address_id);



--
-- Name: aggregated_rewards_validator_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX aggregated_rewards_validator_id_index ON public.aggregated_rewards USING btree (validator_id);



--
-- Name: aggregated_rewards_time_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX aggregated_rewards_time_id_index ON public.aggregated_rewards USING btree (time_id);



--
-- Name: aggregated_rewards_unique_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE UNIQUE INDEX aggregated_rewards_unique_index ON public.aggregated_rewards
    USING btree (time_id, address_id, validator_id, role);



--
-- Name: slashes_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX slashes_address_id_index ON public.slashes USING btree (address_id);


--
-- Name: slashes_block_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX slashes_block_id_index ON public.slashes USING btree (block_id);


--
-- Name: slashes_coin_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX slashes_coin_id_index ON public.slashes USING btree (coin_id);


--
-- Name: slashes_validator_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX slashes_validator_id_index ON public.slashes USING btree (validator_id);


--
-- Name: stakes_coin_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX stakes_coin_id_index ON public.stakes USING btree (coin_id);


--
-- Name: stakes_owner_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX stakes_owner_address_id_index ON public.stakes USING btree (owner_address_id);


--
-- Name: stakes_validator_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX stakes_validator_id_index ON public.stakes USING btree (validator_id);


--
-- Name: transaction_outputs_coin_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX transaction_outputs_coin_id_index ON public.transaction_outputs USING btree (coin_id);


--
-- Name: transaction_outputs_transaction_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX transaction_outputs_transaction_id_index ON public.transaction_outputs USING btree (transaction_id);

--
-- Name: transaction_outputs_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX transaction_outputs_address_id_index ON public.transaction_outputs USING btree (to_address_id);

--
-- Name: transaction_validator_validator_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX transaction_validator_validator_id_index ON public.transaction_validator USING btree (validator_id);


--
-- Name: transactions_block_id_from_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX transactions_block_id_from_address_id_index ON public.transactions USING btree (block_id DESC, from_address_id);


--
-- Name: transactions_from_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX transactions_from_address_id_index ON public.transactions USING btree (from_address_id);


--
-- Name: transactions_hash_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX transactions_hash_index ON public.transactions USING hash (hash);


--
-- Name: validator_public_keys_public_key_uindex; Type: INDEX; Schema: public; Owner: minter
--

CREATE UNIQUE INDEX validator_public_keys_public_key_uindex ON public.validators USING btree (public_key);


--
-- Name: index_transaction_by_address_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX index_transaction_by_address_address_id_index ON public.index_transaction_by_address USING btree (address_id);


--
-- Name: index_transaction_by_address_block_id_address_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX index_transaction_by_address_block_id_address_id_index ON public.index_transaction_by_address USING btree (block_id, address_id);


--
-- Name: index_transaction_by_address_transaction_id_index; Type: INDEX; Schema: public; Owner: minter
--

CREATE INDEX index_transaction_by_address_transaction_id_index ON public.index_transaction_by_address USING btree (transaction_id);


--
-- Name: balances_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.balances
    ADD CONSTRAINT balances_addresses_id_fk FOREIGN KEY (address_id) REFERENCES public.addresses (id);


--
-- Name: balances_coins_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.balances
    ADD CONSTRAINT balances_coins_id_fk FOREIGN KEY (coin_id) REFERENCES public.coins (id);


--
-- Name: block_validator_blocks_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.block_validator
    ADD CONSTRAINT block_validator_blocks_id_fk FOREIGN KEY (block_id) REFERENCES public.blocks (id);


--
-- Name: block_validator_validators_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.block_validator
    ADD CONSTRAINT block_validator_validators_id_fk FOREIGN KEY (validator_id) REFERENCES public.validators (id);

--
-- Name: coins_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.coins
    ADD CONSTRAINT coins_addresses_id_fk FOREIGN KEY (creation_address_id) REFERENCES public.addresses (id);

--
-- Name: coins_transactions_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.coins
    ADD CONSTRAINT coins_transactions_id_fk FOREIGN KEY (creation_transaction_id) REFERENCES public.transactions (id);


--
-- Name: invalid_transactions_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.invalid_transactions
    ADD CONSTRAINT invalid_transactions_addresses_id_fk FOREIGN KEY (from_address_id) REFERENCES public.addresses (id);


--
-- Name: invalid_transactions_blocks_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.invalid_transactions
    ADD CONSTRAINT invalid_transactions_blocks_id_fk FOREIGN KEY (block_id) REFERENCES public.blocks (id);


--
-- Name: rewards_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.rewards
    ADD CONSTRAINT rewards_addresses_id_fk FOREIGN KEY (address_id) REFERENCES public.addresses (id);


--
-- Name: rewards_blocks_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.rewards
    ADD CONSTRAINT rewards_blocks_id_fk FOREIGN KEY (block_id) REFERENCES public.blocks (id);


--
-- Name: rewards_validators_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.rewards
    ADD CONSTRAINT rewards_validators_id_fk FOREIGN KEY (validator_id) REFERENCES public.validators (id);


--
-- Name: aggregated_rewards_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.aggregated_rewards
    ADD CONSTRAINT aggregated_rewards_addresses_id_fk FOREIGN KEY (address_id) REFERENCES public.addresses (id);


--
-- Name: aggregated_rewards_from_blocks_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.aggregated_rewards
    ADD CONSTRAINT aggregated_rewards_from_blocks_id_fk FOREIGN KEY (from_block_id) REFERENCES public.blocks (id);


--
-- Name: aggregated_rewards_to_blocks_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.aggregated_rewards
    ADD CONSTRAINT aggregated_rewards_to_blocks_id_fk FOREIGN KEY (to_block_id) REFERENCES public.blocks (id);


--
-- Name: aggregated_rewards_validators_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.aggregated_rewards
    ADD CONSTRAINT aggregated_rewards_validators_id_fk FOREIGN KEY (validator_id) REFERENCES public.validators (id);


--
-- Name: slashes_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.slashes
    ADD CONSTRAINT slashes_addresses_id_fk FOREIGN KEY (address_id) REFERENCES public.addresses (id);


--
-- Name: slashes_blocks_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.slashes
    ADD CONSTRAINT slashes_blocks_id_fk FOREIGN KEY (block_id) REFERENCES public.blocks (id);


--
-- Name: slashes_coins_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.slashes
    ADD CONSTRAINT slashes_coins_id_fk FOREIGN KEY (coin_id) REFERENCES public.coins (id);


--
-- Name: slashes_validators_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.slashes
    ADD CONSTRAINT slashes_validators_id_fk FOREIGN KEY (validator_id) REFERENCES public.validators (id);


--
-- Name: stakes_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.stakes
    ADD CONSTRAINT stakes_addresses_id_fk FOREIGN KEY (owner_address_id) REFERENCES public.addresses (id);


--
-- Name: stakes_coins_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.stakes
    ADD CONSTRAINT stakes_coins_id_fk FOREIGN KEY (coin_id) REFERENCES public.coins (id);


--
-- Name: stakes_validators_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.stakes
    ADD CONSTRAINT stakes_validators_id_fk FOREIGN KEY (validator_id) REFERENCES public.validators (id);


--
-- Name: transaction_outputs_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transaction_outputs
    ADD CONSTRAINT transaction_outputs_addresses_id_fk FOREIGN KEY (to_address_id) REFERENCES public.addresses (id);


--
-- Name: transaction_outputs_coins_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transaction_outputs
    ADD CONSTRAINT transaction_outputs_coins_id_fk FOREIGN KEY (coin_id) REFERENCES public.coins (id);


--
-- Name: transaction_outputs_transactions_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transaction_outputs
    ADD CONSTRAINT transaction_outputs_transactions_id_fk FOREIGN KEY (transaction_id) REFERENCES public.transactions (id);


--
-- Name: transaction_validator_transactions_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transaction_validator
    ADD CONSTRAINT transaction_validator_transactions_id_fk FOREIGN KEY (transaction_id) REFERENCES public.transactions (id);


--
-- Name: transaction_validator_validators_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transaction_validator
    ADD CONSTRAINT transaction_validator_validators_id_fk FOREIGN KEY (validator_id) REFERENCES public.validators (id);


--
-- Name: transactions_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_addresses_id_fk FOREIGN KEY (from_address_id) REFERENCES public.addresses (id);


--
-- Name: transactions_blocks_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_blocks_id_fk FOREIGN KEY (block_id) REFERENCES public.blocks (id);


--
-- Name: transactions_coins_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_coins_id_fk FOREIGN KEY (gas_coin_id) REFERENCES public.coins (id);


--
-- Name: index_transaction_by_address index_transaction_by_address_addresses_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.index_transaction_by_address
    ADD CONSTRAINT index_transaction_by_address_addresses_id_fk FOREIGN KEY (address_id) REFERENCES public.addresses (id);


--
-- Name: index_transaction_by_address index_transaction_by_address_blocks_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.index_transaction_by_address
    ADD CONSTRAINT index_transaction_by_address_blocks_id_fk FOREIGN KEY (block_id) REFERENCES public.blocks (id);


--
-- Name: index_transaction_by_address index_transaction_by_address_transactions_id_fk; Type: FK CONSTRAINT; Schema: public; Owner: minter
--

ALTER TABLE ONLY public.index_transaction_by_address
    ADD CONSTRAINT index_transaction_by_address_transactions_id_fk FOREIGN KEY (transaction_id) REFERENCES public.transactions (id);


--
-- Name: SCHEMA public; Type: ACL; Schema: -; Owner: minter
--

REVOKE ALL ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM minter;
GRANT ALL ON SCHEMA public TO minter;
GRANT ALL ON SCHEMA public TO PUBLIC;
