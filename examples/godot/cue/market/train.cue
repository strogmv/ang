package market

// Source: content/world/train_motion.cue, exact market binding at simulation epoch 0.
_train: {id: "train.c7.passenger.01", initial_distance_m: 152.4, start_m: 100, x: 0, y: 0, z: 39.5}
placement: extras: [{
  instance_id: _train.id
  model_ref: "market.PassengerTrainV2"
  source_kind: "train_motion_epoch"
  position_plan_m: [_train.x + _train.initial_distance_m - _train.start_m, _train.y, _train.z]
  rotation_degrees: 0
  reason: "Static train epoch sample from authored route distance and the market binding; no runtime train simulation."
}]
