module Api exposing
    ( Body
    , abort
    , command
    , connect
    , disconnect
    , fetchFonts
    , goTo
    , pointer
    , acceptOrigin
    , nudge
    , resetCounter
    , zeroAxes
    , fetchMachines
    , fetchState
    , gridBody
    , hold
    , home
    , probe
    , reset
    , setTool
    , unlock
    , jog
    , labelBody
    , preview
    , resume
    , runJob
    )

import Http
import Json.Decode as D exposing (Decoder)
import Json.Encode as E
import Model exposing (..)



-- REQUESTS


{-| A JSON request body.
-}
type alias Body =
    E.Value


fetchState : Cmd Msg
fetchState =
    Http.get { url = "api/machine/state", expect = expectJson GotState machineStateDecoder }


fetchMachines : Cmd Msg
fetchMachines =
    Http.get { url = "api/machines", expect = expectJson GotMachines (D.list seenMachineDecoder) }


fetchFonts : Cmd Msg
fetchFonts =
    Http.get { url = "api/fonts", expect = expectJson GotFonts (D.list fontDecoder) }


connect : String -> Cmd Msg
connect host =
    post "api/machine/connect" (E.object [ ( "host", E.string host ) ]) (expectJson GotAction machineStateDecoder)


disconnect : Cmd Msg
disconnect =
    action "api/machine/disconnect"


abort : Cmd Msg
abort =
    action "api/machine/abort"


hold : Cmd Msg
hold =
    action "api/machine/hold"


resume : Cmd Msg
resume =
    action "api/machine/resume"


home : Cmd Msg
home =
    action "api/machine/home"


unlock : Cmd Msg
unlock =
    action "api/machine/unlock"


reset : Cmd Msg
reset =
    action "api/machine/reset"


setTool : String -> Cmd Msg
setTool tool =
    post "api/machine/tool" (E.object [ ( "tool", E.string tool ) ]) (expectJson GotAction machineStateDecoder)


zeroAxes : String -> Cmd Msg
zeroAxes axes =
    post "api/machine/origin" (E.object [ ( "axes", E.string axes ) ]) (expectJson GotAction machineStateDecoder)


acceptOrigin : Cmd Msg
acceptOrigin =
    post "api/machine/origin" (E.object [ ( "accept", E.bool True ) ]) (expectJson GotAction machineStateDecoder)


{-| Nudge the origin; with a workpiece size, re-trace it afterwards.
-}
nudge : Float -> Float -> Maybe ( Float, Float, Float ) -> Cmd Msg
nudge dx dy retrace =
    let
        retraceField =
            case retrace of
                Just ( w, h, m ) ->
                    [ ( "retrace", E.object [ ( "widthMM", E.float w ), ( "heightMM", E.float h ), ( "marginMM", E.float m ) ] ) ]

                Nothing ->
                    []
    in
    post "api/machine/origin"
        (E.object ([ ( "dx", E.float dx ), ( "dy", E.float dy ) ] ++ retraceField))
        (expectJson GotAction machineStateDecoder)


pointer : Bool -> Cmd Msg
pointer on =
    post "api/machine/pointer" (E.object [ ( "on", E.bool on ) ]) (expectJson GotAction machineStateDecoder)


resetCounter : Cmd Msg
resetCounter =
    action "api/machine/counter/reset"


probe : Float -> Float -> Float -> Bool -> Bool -> Cmd Msg
probe width height margin outline z =
    post "api/machine/probe"
        (E.object
            [ ( "widthMM", E.float width )
            , ( "heightMM", E.float height )
            , ( "marginMM", E.float margin )
            , ( "outline", E.bool outline )
            , ( "z", E.bool z )
            ]
        )
        (expectJson GotAction machineStateDecoder)


command : String -> Cmd Msg
command line =
    post "api/machine/cmd"
        (E.object [ ( "line", E.string line ) ])
        (expectJson GotCommand (D.field "lines" (D.list D.string)))


goTo : String -> Cmd Msg
goTo target =
    post "api/machine/goto" (E.object [ ( "target", E.string target ) ]) (expectJson GotAction machineStateDecoder)


jog : String -> Float -> Cmd Msg
jog axis mm =
    post "api/machine/jog"
        (E.object [ ( "axis", E.string axis ), ( "mm", E.float mm ) ])
        (expectJson GotAction machineStateDecoder)


preview : Tab -> Int -> E.Value -> Cmd Msg
preview tab seq body =
    post (endpoint "preview" tab) body (expectJson (GotPreview tab seq) previewDecoder)


runJob : Tab -> E.Value -> Cmd Msg
runJob tab body =
    post (endpoint "job" tab) body (expectJson GotAction machineStateDecoder)


endpoint : String -> Tab -> String
endpoint kind tab =
    case tab of
        GridTab ->
            "api/" ++ kind ++ "/testgrid"

        _ ->
            "api/" ++ kind ++ "/label"


action : String -> Cmd Msg
action url =
    post url (E.object []) (expectJson GotAction machineStateDecoder)


post : String -> E.Value -> Http.Expect Msg -> Cmd Msg
post url body expect =
    Http.post { url = url, body = Http.jsonBody body, expect = expect }


{-| Like Http.expectJson, but reports the server's error message.
-}
expectJson : (Result String a -> Msg) -> Decoder a -> Http.Expect Msg
expectJson toMsg decoder =
    Http.expectStringResponse toMsg <|
        \response ->
            case response of
                Http.GoodStatus_ _ body ->
                    D.decodeString decoder body |> Result.mapError D.errorToString

                Http.BadStatus_ meta body ->
                    Err
                        (D.decodeString (D.field "error" D.string) body
                            |> Result.withDefault ("HTTP " ++ String.fromInt meta.statusCode)
                        )

                Http.Timeout_ ->
                    Err "request timed out"

                Http.NetworkError_ ->
                    Err "server unreachable"

                Http.BadUrl_ url ->
                    Err ("bad URL " ++ url)



-- REQUEST BODIES


gridBody : String -> GridForm -> Result String E.Value
gridBody framing g =
    Result.map5
        (\powers feeds cell gap interval ->
            E.object
                [ ( "powers", E.list E.float powers )
                , ( "feeds", E.list E.float feeds )
                , ( "cellMM", E.float cell )
                , ( "gapMM", E.float gap )
                , ( "hatchIntervalMM", E.float interval )
                , ( "framing", E.string framing )
                ]
        )
        (g.powers |> List.map (percent "row power") |> combine)
        (g.feeds |> List.map (float "column feed") |> combine)
        (float "cell size" g.cell)
        (float "gap" g.gap)
        (float "interval" g.interval)


labelBody : String -> LabelForm -> LaserForm -> Result String E.Value
labelBody framing l s =
    let
        lines =
            l.lines
                |> List.map
                    (\line ->
                        float "text size" line.size
                            |> Result.map
                                (\size ->
                                    E.object
                                        [ ( "text", E.string line.text )
                                        , ( "fontID", E.string l.fontID )
                                        , ( "sizeMM", E.float size )
                                        , ( "align", E.string l.align )
                                        ]
                                )
                    )
                |> combine

        labelValue =
            Result.map2
                (\ls spacing -> E.object [ ( "lines", E.list identity ls ), ( "lineSpacing", E.float spacing ), ( "fit", E.bool l.fit ) ])
                lines
                (float "line spacing" l.spacing)

        workpiece =
            Result.map3
                (\w h m -> E.object [ ( "widthMM", E.float w ), ( "heightMM", E.float h ), ( "marginMM", E.float m ) ])
                (float "width" l.width)
                (float "height" l.height)
                (float "margin" l.margin)
    in
    if l.fontID == "" then
        Err "choose a font"

    else
        Result.map3
            (\lab wp st ->
                E.object
                    [ ( "label", lab )
                    , ( "workpiece", wp )
                    , ( "settings", st )
                    , ( "framing", E.string framing )
                    ]
            )
            labelValue
            workpiece
            (settingsBody s)


settingsBody : LaserForm -> Result String E.Value
settingsBody s =
    Result.map5
        (\power feed passes interval angle ->
            E.object
                [ ( "power", E.float power )
                , ( "feed", E.float feed )
                , ( "passes", E.int passes )
                , ( "mode", E.string s.mode )
                , ( "hatchIntervalMM", E.float interval )
                , ( "hatchAngleDeg", E.float angle )
                , ( "bidirectional", E.bool s.bidir )
                ]
        )
        (percent "power" s.power)
        (float "feed" s.feed)
        (String.toInt (String.trim s.passes) |> Result.fromMaybe "passes must be a whole number")
        (float "interval" s.interval)
        (float "angle" s.angle)


float : String -> String -> Result String Float
float name s =
    String.toFloat (String.trim s) |> Result.fromMaybe (name ++ " must be a number")


{-| Parse a percentage into the 0..1 range used by the machine.
-}
percent : String -> String -> Result String Float
percent name s =
    float name s |> Result.map (\p -> p / 100)


combine : List (Result e a) -> Result e (List a)
combine =
    List.foldr (Result.map2 (::)) (Ok [])



-- DECODERS


machineStateDecoder : Decoder MachineState
machineStateDecoder =
    D.succeed MachineState
        |> andMap (D.field "connected" D.bool)
        |> andMap (D.field "addr" D.string)
        |> andMap (D.maybe (D.field "error" D.string))
        |> andMap (D.field "status" (D.nullable statusDecoder))
        |> andMap (D.field "job" (D.nullable jobDecoder))
        |> andMap (D.field "log" (D.list D.string))
        |> andMap (D.field "homed" D.bool)
        |> andMap (D.maybe (D.field "halt" D.string))
        |> andMap (D.field "needsReset" D.bool)
        |> andMap (D.field "originSet" D.bool)
        |> andMap (D.field "offsets" (D.nullable offsetsDecoder))
        |> andMap (D.field "traced" D.bool)
        |> andMap (D.field "zProbed" D.bool)
        |> andMap (D.field "labelsDone" D.int)


seenMachineDecoder : Decoder SeenMachine
seenMachineDecoder =
    D.map5 (\name ip portNumber busy state -> SeenMachine name (ip ++ ":" ++ String.fromInt portNumber) busy state)
        (D.field "name" D.string)
        (D.field "ip" D.string)
        (D.field "port" D.int)
        (D.field "busy" D.bool)
        (D.field "state" D.string)


offsetsDecoder : Decoder Offsets
offsetsDecoder =
    D.map3 Offsets
        (D.field "x" D.float)
        (D.field "y" D.float)
        (D.field "z" D.float)


statusDecoder : Decoder Status
statusDecoder =
    D.map8 Status
        (D.field "state" D.string)
        (D.field "wpos" (D.list D.float))
        (D.field "mpos" (D.list D.float))
        (D.field "tool" D.int)
        (D.field "laserMode" D.bool)
        (D.field "laserOn" D.bool)
        (D.field "playing" D.bool)
        (D.field "progress" (D.list D.int))


jobDecoder : Decoder Job
jobDecoder =
    D.map5 Job
        (D.field "name" D.string)
        (D.field "phase" D.string)
        (D.field "sent" D.int)
        (D.field "total" D.int)
        (D.maybe (D.field "error" D.string))


fontDecoder : Decoder FontInfo
fontDecoder =
    D.map3 FontInfo
        (D.field "id" D.string)
        (D.field "family" D.string)
        (D.field "style" D.string)


rectDecoder : Decoder Rect
rectDecoder =
    D.map4 Rect
        (D.at [ "min", "x" ] D.float)
        (D.at [ "min", "y" ] D.float)
        (D.at [ "max", "x" ] D.float)
        (D.at [ "max", "y" ] D.float)


segDecoder : Decoder Seg
segDecoder =
    D.map4 Seg
        (D.index 0 D.float)
        (D.index 1 D.float)
        (D.index 2 D.float)
        (D.index 3 D.float)


previewDecoder : Decoder Preview
previewDecoder =
    D.succeed Preview
        |> andMap (D.field "burn" (D.list segDecoder))
        |> andMap (D.field "travel" (D.list segDecoder))
        |> andMap (D.field "bounds" rectDecoder)
        |> andMap (D.field "workpiece" (D.nullable rectDecoder))
        |> andMap (D.field "printable" (D.nullable rectDecoder))
        |> andMap (D.field "warnings" (D.list warningDecoder))
        |> andMap (D.field "burnMM" D.float)
        |> andMap (D.field "travelMM" D.float)
        |> andMap (D.field "estimateSec" D.float)
        |> andMap (D.field "scale" D.float)
        |> andMap (D.field "lines" D.int)
        |> andMap (D.field "gcode" D.string)


warningDecoder : Decoder Warning
warningDecoder =
    D.map2 Warning
        (D.field "code" D.string)
        (D.field "message" D.string)


andMap : Decoder a -> Decoder (a -> b) -> Decoder b
andMap =
    D.map2 (|>)
