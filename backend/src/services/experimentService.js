const { agents, attackFamilies, experimentLayers, buildExperimentPlan, scenarioTemplates } = require("../data/catalog");

function listAgents() {
  return agents;
}

function listAttackFamilies() {
  return attackFamilies;
}

function listExperimentLayers() {
  return experimentLayers;
}

function getExperimentPlan() {
  return buildExperimentPlan();
}

function getScenarioTemplates() {
  return scenarioTemplates;
}

module.exports = {
  listAgents,
  listAttackFamilies,
  listExperimentLayers,
  getExperimentPlan,
  getScenarioTemplates
};
