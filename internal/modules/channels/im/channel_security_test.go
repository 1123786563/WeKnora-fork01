package im

import "testing"

func TestValidateChannelTransportRequiresDingTalkStream(t *testing.T) {
	cases := []struct {
		name    string
		channel *IMChannel
		wantErr bool
	}{
		{"dingtalk websocket ok", &IMChannel{Platform: "dingtalk", Mode: "websocket"}, false},
		{"dingtalk default mode is stream", &IMChannel{Platform: "dingtalk"}, false},
		{"dingtalk http callback refused", &IMChannel{Platform: "dingtalk", Mode: "http"}, true},
		{
			name:    "other platforms unrestricted",
			channel: &IMChannel{Platform: "feishu", Mode: "http"},
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateChannelTransport(tc.channel)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
