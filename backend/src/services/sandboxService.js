const { simpleHash } = require("../lib/crypto");

function runMemorySandbox(rawText) {
  const markers = [
    /ignore all safety rules/ig,
    /ignore previous instructions/ig,
    /remember this command forever/ig,
    /写入长期记忆/ig,
    /忽略规则/ig
  ];

  let cleaned = String(rawText);
  let hitCount = 0;

  markers.forEach((pattern) => {
    if (pattern.test(cleaned)) {
      hitCount += 1;
      cleaned = cleaned.replace(pattern, "[filtered]");
    }
  });

  return {
    fingerprint_sm3: simpleHash(rawText),
    source_tag: "untrusted_external_result",
    blocked_markers: hitCount,
    trusted_summary: cleaned.length > 240 ? `${cleaned.slice(0, 240)}...` : cleaned
  };
}

module.exports = {
  runMemorySandbox
};
