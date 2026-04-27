# openav microservice kramer switcher

OpenAV microservice for the Kramer Switcher.  Uses the microservice framework and Kramer Protocol 3000 for control communication.

A Kramer Switcher is a hardware device used for managing and routing multiple audio, video, and control signals between different sources and displays.

[Kramer VP-558 boardroom switcher scaler](https://www1.kramerav.com/us/product/vp-558)

[Microservice curl test documentation](https://github.com/Dartmouth-OpenAV/documentation/blob/main/curl_test_readme.md)

## Microservice-specific Endpoint Notes and Exceptions:

- `audiomute` is not implemented for VP-558 (would be a lot of work to implement it), but we do use it for VP-440H2 in rooms that don’t have a DSP. VP-558 is an old product, and we expect that a room with a video switcher that big will also have a DSP for audio mute control.

- Volume values: If you specify `name >= 1000` these functions will set or get the output stage channel calculated as `(name - 1000)` instead of setting the input channel specified. So for the VP-440H2 output with its single analog output, the output name is always `1000`.

- Input and output values for VP-440H2: This is very similar to videoinputstatus, but it formats the response differently

```text
output – 1 (HDMI OUT)
input – 0 (HDMI IN 1), 1 (HDMI IN 2), 2 (HDMI IN 3), 3 (HDBT IN), 4 (PC IN)
```

- Input and output values for VP-558:

```text
output: 1 (Output1),  2 (Output2), 3 (Output3) 4 (Output4)
input: 1 (HDMI1) 2 (HDMI2) 3 (HDMI3) 4 (HDMI4) 5 (HDMI5) 6 (HDMI6) 7 (HDBT1) 8 (HDBT2) 9 (HDBT3) 10 (HDBT4 11 (PC)
```

- `videoroute` and `audioandvideoroute` *output* params: Numbering is the same as Volume, but don't add `1000` for Kramer devices

- `occupancystatus`: Return is very similar to `videoinputstatus`, but the return is formatted differently

![](https://github.com/Dartmouth-OpenAV/microservice-kramer-switcher/blob/main/photo.png)

![](https://github.com/Dartmouth-OpenAV/microservice-kramer-switcher/blob/main/diagram.png)
