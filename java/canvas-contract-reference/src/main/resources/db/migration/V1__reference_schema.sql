create table if not exists reference_state (
    id bigint not null primary key,
    state_json clob not null,
    version bigint not null
);
