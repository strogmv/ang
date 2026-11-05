package gamepoc

import "testing"

func TestTrainReviewMeasuresRenderedPosesNotInactiveCachedHierarchy(t *testing.T) {
	helper := sceneTemplateSection(t, "train_motion_proof.cs.tmpl", "  static double TrainReviewMeasuredError(", "  bool TrainReviewOnScreen(")
	runPaintedTextureNative(t, "TrainReviewPoseScope", `using System;
 class Proof {`+helper+`
 static void Need(bool condition,string reason){if(!condition)throw new Exception(reason);}
 public static void Main(){
 double measured=TrainReviewMeasuredError(true,true,.008,0);
 Need(measured==.008,"live rendered pose was not measured");
 measured=TrainReviewMeasuredError(false,false,325.6875,measured);
 Need(measured==.008,"hidden cached pose at distant route connector changed visible error");
 measured=TrainReviewMeasuredError(true,false,325.6875,measured);
 Need(measured==.008,"inactive parent hierarchy was mistaken for an active child");
 measured=TrainReviewMeasuredError(false,true,9,measured);
 Need(measured==.008,"binding outside visible rail was measured as rendered");
 measured=TrainReviewMeasuredError(true,true,.03,measured);
 Need(measured>.01,"real active pose failure was hidden or tolerance weakened");
 Console.WriteLine("active visible poses measured; cached inactive route connectors excluded; strict actual pose failure retained PASS");
 }} `)
}
