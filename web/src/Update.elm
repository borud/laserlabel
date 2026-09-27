module Update exposing (update)

import Api
import Model exposing (..)
import Process
import Task


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    let
        ( next, cmd ) =
            updateModel msg model
    in
    if next.notice /= Nothing && next.notice /= model.notice then
        ( { next | noticeSeq = next.noticeSeq + 1 }
        , Cmd.batch [ cmd, Process.sleep 5000 |> Task.perform (\_ -> ExpireNotice (next.noticeSeq + 1)) ]
        )

    else
        ( next, cmd )


updateModel : Msg -> Model -> ( Model, Cmd Msg )
updateModel msg model =
    case msg of
        SetTab tab ->
            refreshPreview { model | tab = tab }

        Tick _ ->
            ( model, Cmd.batch [ Api.fetchState, Api.fetchMachines ] )

        GotMachines (Ok ms) ->
            ( { model | machines = ms }, Cmd.none )

        GotMachines (Err _) ->
            ( model, Cmd.none )

        ConnectTo addr ->
            ( { model | notice = Nothing }, Api.connect addr )

        GotState (Ok st) ->
            ( { model | machine = Just st }, Cmd.none )

        GotState (Err _) ->
            ( { model | machine = Nothing }, Cmd.none )

        GotAction (Ok st) ->
            ( { model | machine = Just st }, Cmd.none )

        GotAction (Err e) ->
            -- Busy replies come from taps while a move is still running;
            -- the buttons already show that, so don't pop up a message.
            if e == "machine is busy" then
                ( model, Cmd.none )

            else
                ( { model | notice = Just e }, Cmd.none )

        GotFonts (Ok fonts) ->
            let
                label =
                    model.label

                fontID =
                    if label.fontID == "" then
                        defaultFont fonts

                    else
                        label.fontID
            in
            refreshPreview { model | fonts = Success fonts, label = { label | fontID = fontID } }

        GotFonts (Err e) ->
            ( { model | fonts = Failure e }, Cmd.none )

        SetHost h ->
            ( { model | host = h }, Cmd.none )

        Connect ->
            ( { model | notice = Nothing }, Api.connect model.host )

        Disconnect ->
            ( model, Api.disconnect )

        SetConsole s ->
            ( { model | console = s }, Cmd.none )

        SendConsole ->
            if String.trim model.console == "" then
                ( model, Cmd.none )

            else
                ( { model | console = "" }, Api.command model.console )

        GotCommand (Ok _) ->
            ( model, Api.fetchState )

        GotCommand (Err e) ->
            ( { model | notice = Just e }, Cmd.none )

        SetPadMode mode ->
            ( { model | padMode = mode }, Cmd.none )

        SetPadStep step ->
            case model.padMode of
                MoveHead ->
                    ( { model | jogStep = step }, Cmd.none )

                MoveOrigin ->
                    ( { model | nudgeStep = step }, Cmd.none )

        Pad axis dir ->
            case model.padMode of
                MoveHead ->
                    ( model, Api.jog axis (dir * model.jogStep) )

                MoveOrigin ->
                    let
                        d =
                            dir * model.nudgeStep

                        ( dx, dy ) =
                            if axis == "X" then
                                ( d, 0 )

                            else
                                ( 0, d )
                    in
                    ( { model | notice = Nothing }, Api.nudge dx dy (retraceSize model) )

        ToggleRetrace ->
            ( { model | retrace = not model.retrace }, Cmd.none )

        SetPointer on ->
            ( { model | pointer = on }, Api.pointer on )

        ZeroAxes axes ->
            ( model, Api.zeroAxes axes )

        ResetCounter ->
            ( model, Api.resetCounter )

        GoTo target ->
            ( { model | notice = Nothing }, Api.goTo target )

        Abort ->
            ( model, Api.abort )

        Hold ->
            ( model, Api.hold )

        Resume ->
            ( model, Api.resume )

        SetField field value ->
            refreshPreview (setField field value model)

        SetLineText i s ->
            refreshPreview (updateLines (updateAt i (\l -> { l | text = s })) model)

        SetLineSize i s ->
            refreshPreview (updateLines (updateAt i (\l -> { l | size = s })) model)

        AddLine ->
            refreshPreview (updateLines (\ls -> ls ++ [ { text = "", size = "6" } ]) model)

        RemoveLine i ->
            refreshPreview (updateLines (removeAt i) model)

        SetFont id ->
            let
                label =
                    model.label
            in
            refreshPreview { model | label = { label | fontID = id } }

        ToggleFit ->
            let
                label =
                    model.label
            in
            refreshPreview { model | label = { label | fit = not label.fit } }

        ToggleBidir ->
            let
                laser =
                    model.laser
            in
            refreshPreview { model | laser = { laser | bidir = not laser.bidir } }

        SetFraming f ->
            ( { model | framing = f }, Cmd.none )

        GotPreview tab seq result ->
            if seq /= model.previewSeq then
                ( model, Cmd.none )

            else
                ( setPreview tab (fromResult result) model, Cmd.none )

        RunJob tab ->
            case body tab model of
                Ok b ->
                    ( { model | notice = Nothing }, Api.runJob tab b )

                Err e ->
                    ( { model | notice = Just e }, Cmd.none )

        ToggleTravel ->
            ( { model | showTravel = not model.showTravel }, Cmd.none )

        ToggleGcode ->
            ( { model | showGcode = not model.showGcode }, Cmd.none )

        DismissNotice ->
            ( { model | notice = Nothing }, Cmd.none )

        ExpireNotice seq ->
            if seq == model.noticeSeq then
                ( { model | notice = Nothing }, Cmd.none )

            else
                ( model, Cmd.none )

        Home ->
            ( model, Api.home )

        Unlock ->
            ( model, Api.unlock )

        Reset ->
            ( model, Api.reset )

        SetTool tool ->
            ( model, Api.setTool tool )

        AcceptOrigin ->
            ( model, Api.acceptOrigin )

        Probe outline z ->
            case ( String.toFloat model.label.width, String.toFloat model.label.height, String.toFloat model.label.margin ) of
                ( Just w, Just h, Just margin ) ->
                    ( { model | notice = Nothing }, Api.probe w h margin outline z )

                _ ->
                    ( { model | notice = Just "set a valid workpiece size on the Label tab" }, Cmd.none )

        SetGridPower i v ->
            refreshPreview (updateGrid (\g -> { g | powers = updateAt i (always v) g.powers }) model)

        AddGridPower ->
            refreshPreview (updateGrid (\g -> { g | powers = g.powers ++ [ nextValue 10 g.powers ] }) model)

        RemoveGridPower i ->
            refreshPreview (updateGrid (\g -> { g | powers = removeAt i g.powers }) model)

        SetGridFeed i v ->
            refreshPreview (updateGrid (\g -> { g | feeds = updateAt i (always v) g.feeds }) model)

        AddGridFeed ->
            refreshPreview (updateGrid (\g -> { g | feeds = g.feeds ++ [ nextValue 500 g.feeds ] }) model)

        RemoveGridFeed i ->
            refreshPreview (updateGrid (\g -> { g | feeds = removeAt i g.feeds }) model)


{-| The workpiece size to re-trace after an origin nudge, if enabled.
-}
retraceSize : Model -> Maybe ( Float, Float, Float )
retraceSize model =
    if model.retrace then
        Maybe.map3 (\w h m -> ( w, h, m ))
            (String.toFloat model.label.width)
            (String.toFloat model.label.height)
            (String.toFloat model.label.margin)

    else
        Nothing


{-| Request a new preview for the current tab. Responses to older
requests are ignored using the sequence number.
-}
refreshPreview : Model -> ( Model, Cmd Msg )
refreshPreview model =
    if model.tab /= LabelTab && model.tab /= GridTab then
        ( model, Cmd.none )

    else
        let
            seq =
                model.previewSeq + 1

            next =
                { model | previewSeq = seq }
        in
        case body model.tab model of
            Ok b ->
                ( setPreview model.tab Loading next, Api.preview model.tab seq b )

            Err e ->
                ( setPreview model.tab (Failure e) next, Cmd.none )


body : Tab -> Model -> Result String Api.Body
body tab model =
    case tab of
        GridTab ->
            Api.gridBody model.framing model.grid

        _ ->
            Api.labelBody model.framing model.label model.laser


setPreview : Tab -> RemoteData Preview -> Model -> Model
setPreview tab p model =
    case tab of
        GridTab ->
            { model | gridPreview = p }

        LabelTab ->
            { model | labelPreview = p }

        _ ->
            model


fromResult : Result String a -> RemoteData a
fromResult r =
    case r of
        Ok a ->
            Success a

        Err e ->
            Failure e


setField : Field -> String -> Model -> Model
setField field v model =
    let
        g =
            model.grid

        l =
            model.label

        s =
            model.laser
    in
    case field of
        GridCell ->
            { model | grid = { g | cell = v } }

        GridGap ->
            { model | grid = { g | gap = v } }

        GridInterval ->
            { model | grid = { g | interval = v } }

        LabelFontFilter ->
            { model | label = { l | fontFilter = v } }

        LabelAlign ->
            { model | label = { l | align = v } }

        LabelSpacing ->
            { model | label = { l | spacing = v } }

        LabelWidth ->
            { model | label = { l | width = v } }

        LabelHeight ->
            { model | label = { l | height = v } }

        LabelMargin ->
            { model | label = { l | margin = v } }

        LaserPower ->
            { model | laser = { s | power = v } }

        LaserFeed ->
            { model | laser = { s | feed = v } }

        LaserPasses ->
            { model | laser = { s | passes = v } }

        LaserMode ->
            { model | laser = { s | mode = v } }

        LaserInterval ->
            { model | laser = { s | interval = v } }

        LaserAngle ->
            { model | laser = { s | angle = v } }


updateGrid : (GridForm -> GridForm) -> Model -> Model
updateGrid f model =
    { model | grid = f model.grid }


{-| Suggest the next value in a series: the last value plus step.
-}
nextValue : Float -> List String -> String
nextValue step values =
    values
        |> List.reverse
        |> List.head
        |> Maybe.andThen String.toFloat
        |> Maybe.map (\v -> String.fromFloat (v + step))
        |> Maybe.withDefault (String.fromFloat step)


updateLines : (List LineForm -> List LineForm) -> Model -> Model
updateLines f model =
    let
        l =
            model.label
    in
    { model | label = { l | lines = f l.lines } }


updateAt : Int -> (a -> a) -> List a -> List a
updateAt i f =
    List.indexedMap
        (\j x ->
            if i == j then
                f x

            else
                x
        )


removeAt : Int -> List a -> List a
removeAt i xs =
    List.take i xs ++ List.drop (i + 1) xs


{-| Prefer a bold sans-serif that is likely to exist.
-}
defaultFont : List FontInfo -> String
defaultFont fonts =
    let
        find family style =
            List.filter (\f -> f.family == family && f.style == style) fonts |> List.head
    in
    [ find "Helvetica" "Bold"
    , find "Arial" "Bold"
    , find "Liberation Sans" "Bold"
    , find "DejaVu Sans" "Bold"
    , List.head fonts
    ]
        |> List.filterMap identity
        |> List.head
        |> Maybe.map .id
        |> Maybe.withDefault ""
