CREATE TABLE IF NOT EXISTS Article (
    url TEXT PRIMARY KEY NOT NULL,
    title TEXT NOT NULL 
);

CREATE TABLE IF NOT EXISTS User (
    id INTEGER PRIMARY KEY NOT NULL,
    email TEXT NOT NULL,
    screename TEXT NOT NULL,
    passwordhash TEXT NOT NULL,
    UNIQUE(email, screename)
);

CREATE TABLE IF NOT EXISTS Revision (
    id INTEGER PRIMARY KEY NOT NULL,
    hashval TEXT NOT NULL,
    markdown TEXT NOT NULL,
    rawhtml TEXT NOT NULL,
    article_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    UNIQUE(hashval),
    FOREIGN KEY(article_id) REFERENCES Article(id),
    FOREIGN KEY(user_id) REFERENCES User(id)
);