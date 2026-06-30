db = db.getSiblingDB("app");
db.events.insertMany([
  { type: "signup", user: "ada" },
  { type: "login", user: "alan" },
  { type: "signup", user: "grace" },
]);
